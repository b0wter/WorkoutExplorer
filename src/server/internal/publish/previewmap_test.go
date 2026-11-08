package publish

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
	pagevm "workout-explorer/internal/page"
	"workout-explorer/internal/preview"
	"workout-explorer/internal/workout"
)

// F-36 MC-3 (validity = key match in every build) and MC-6's fallbacks
// (ADR-041). Synthetic files and basemaps only.

type fakeMaps map[string][]byte // key → stored map

func (m fakeMaps) PreviewMap(_, key string) ([]byte, error) {
	b, ok := m[key]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist}
	}
	return b, nil
}

func basemap(t *testing.T, noise bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, preview.Width, preview.Height))
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < len(img.Pix); i += 4 {
		c := color.RGBA{0xe8, 0xe4, 0xd8, 0xff}
		if i/4%preview.Width > 800 {
			c = color.RGBA{0xaa, 0xd3, 0xdf, 0xff}
		}
		if noise {
			c = color.RGBA{uint8(r.Uint32()), uint8(r.Uint32()), uint8(r.Uint32()), 0xff}
		}
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// served is the preview the page links and the public data file.
func (fx *fixture) served(t *testing.T) ([]byte, datafile.File, string) {
	t.Helper()
	name, page := fx.complete(t)
	bin, err := os.ReadFile(filepath.Join(fx.folder(), name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	img, err := os.ReadFile(filepath.Join(fx.folder(), previewOf(t, page)))
	if err != nil {
		t.Fatal(err)
	}
	m := ogTitle.FindSubmatch(page)
	return img, f, string(m[1])
}

// serverCard is F-23's card for the served file and title.
func serverCard(t *testing.T, f datafile.File, title string) []byte {
	t.Helper()
	b, err := preview.Render(f, pagevm.Build(f, pagevm.Settings{Title: title}))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func isMapCard(t *testing.T, b []byte) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := img.(*image.Paletted)
	return ok
}

// movingParser moves the first position by dLat: the parser seam of AC 43.
type movingParser struct {
	inner Parser
	dLat  float64
}

func (p movingParser) Parse(ctx context.Context, b []byte) (workout.Workout, error) {
	w, err := p.inner.Parse(ctx, b)
	if err == nil && p.dLat != 0 {
		w.Samples.Lat = slices.Clone(w.Samples.Lat)
		for i, v := range w.Samples.Lat {
			if v == v { // the first valid one
				w.Samples.Lat[i] += p.dLat
				break
			}
		}
	}
	return w, err
}

// TestMapCardKeyBinding (AC 2, 42, 43; MC-3): a stored map is used only when
// its key equals the frame of the build's own public file. Hiding on, or a
// parser that moves one position, falls back to F-23's card; a title change
// keeps the map and redraws the text.
func TestMapCardKeyBinding(t *testing.T) {
	ctx := context.Background()
	fx := newFixture(t, "ride-30min.fit")
	maps := fakeMaps{}
	fx.a.d.Maps = maps
	off := published(draft())

	frOff, ok, err := fx.a.Frame(ctx, off)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	maps[frOff.Key] = basemap(t, false)
	fx.change(t, draft(), off)
	img, f, title := fx.served(t)
	if !isMapCard(t, img) || bytes.Equal(img, serverCard(t, f, title)) || len(img) > preview.MaxBytes {
		t.Fatal("hiding off: the stored map was not used")
	}
	if fr, _ := preview.FrameOf(f); fr != frOff {
		t.Error("Frame is not the frame of the public file")
	}

	// Hiding on: a new frame from the public file only; the old map is not used.
	on := off
	on.HideEnds = true
	frOn, _, _ := fx.a.Frame(ctx, on)
	fx.change(t, off, on)
	img, f, title = fx.served(t)
	if fr, _ := preview.FrameOf(f); fr != frOn || frOn.Key == frOff.Key {
		t.Errorf("hiding on: frame %+v, public file's %+v, hiding off %+v", frOn, fr, frOff)
	}
	if !bytes.Equal(img, serverCard(t, f, title)) {
		t.Error("hiding on: a map stored for the hiding-off frame was used")
	}

	// Hiding off again, then a title change: map kept, text redrawn.
	fx.change(t, on, off)
	before, _, _ := fx.served(t)
	renamed := off
	renamed.Title = "Renamed loop"
	fx.change(t, off, renamed)
	img, f, title = fx.served(t)
	if !isMapCard(t, before) || !isMapCard(t, img) || bytes.Equal(img, before) || title != "Renamed loop" {
		t.Error("title change: map not kept or text not redrawn")
	}

	// A parser change that moves one position: another key, F-23's card.
	fx.a.d.Parser = movingParser{inner: fx.a.d.Parser, dLat: 0.05}
	fx.change(t, renamed, renamed)
	img, f, title = fx.served(t)
	if !bytes.Equal(img, serverCard(t, f, title)) {
		t.Error("parser seam: a map for the old frame was used")
	}
}

// TestMapCardFallbacks (MC-6): a map whose card exceeds 300 KB, or a stored
// map that does not decode, gives F-23's card; the log names the reason,
// never the key.
func TestMapCardFallbacks(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name, reason string
		data         []byte
	}{
		{"noise", "size", basemap(t, true)},
		{"broken", "render", []byte("not a png")},
	} {
		fx := newFixture(t, "ride-30min.fit")
		var logs bytes.Buffer
		fx.a.d.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
		pub := published(draft())
		fr, _, _ := fx.a.Frame(ctx, pub)
		fx.a.d.Maps = fakeMaps{fr.Key: c.data}
		fx.change(t, draft(), pub)
		img, f, title := fx.served(t)
		if !bytes.Equal(img, serverCard(t, f, title)) {
			t.Errorf("%s: not F-23's card", c.name)
		}
		if !strings.Contains(logs.String(), `"msg":"preview map not used","id":"`+id+`","reason":"`+c.reason+`"`) || strings.Contains(logs.String(), fr.Key) {
			t.Errorf("%s: log %s", c.name, logs.String())
		}
	}
}

// TestFrameWritesNothing (MC-4): the GET's front half leaves public/ alone;
// no positions give no frame.
func TestFrameWritesNothing(t *testing.T) {
	ctx := context.Background()
	fx := newFixture(t, "indoor-no-gps.fit")
	if _, ok, err := fx.a.Frame(ctx, draft()); ok || err != nil {
		t.Errorf("indoor: %v %v", ok, err)
	}
	fx = newFixture(t, "ride-30min.fit")
	if _, ok, err := fx.a.Frame(ctx, published(draft())); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if es, _ := os.ReadDir(fx.root); len(es) != 0 {
		t.Errorf("Frame wrote %v", es)
	}
}
