package preview

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
	"workout-explorer/internal/workout"
)

// F-36 MC-2, MC-5, MC-6 (ADR-041). Synthetic files and synthetic basemaps only.

// TestFrameProjection: the projection is MapLibre's Web Mercator with a
// 512 px world, centred on the canvas.
func TestFrameProjection(t *testing.T) {
	for _, c := range []struct {
		fr   Frame
		p    [2]float64
		x, y float64
	}{
		{Frame{}, [2]float64{0, 0}, 600, 315},
		{Frame{}, [2]float64{0, 90}, 728, 315},
		{Frame{Zoom: 1}, [2]float64{0, -90}, 344, 315},
		{Frame{Lon: 8, Lat: 50, Zoom: 10}, [2]float64{50, 8}, 600, 315},
		{Frame{Zoom: 0}, [2]float64{maxMercLat, 0}, 600, 315 - 256},
	} {
		x, y := c.fr.project(c.p)
		if math.Abs(x-c.x) > 1e-6 || math.Abs(y-c.y) > 1e-3 {
			t.Errorf("%+v %v: (%g, %g), want (%g, %g)", c.fr, c.p, x, y, c.x, c.y)
		}
	}
}

// TestFrame: the route's box fills the map area (MC-2), rounded values,
// a 16-hex key that is stable and follows the positions; no positions, no frame.
func TestFrame(t *testing.T) {
	for _, name := range []string{"ride-30min.fit", "run-30min.fit", "ride-12h.fit", "gps-dropout.fit"} {
		f := public(t, parse(t, name))
		fr, ok := FrameOf(f)
		if !ok || !ValidKey(fr.Key) || fr.Zoom != math.Floor(fr.Zoom*100)/100 || fr.Lon != round(fr.Lon, 6) || fr.Lat != round(fr.Lat, 6) {
			t.Fatalf("%s: %+v %v", name, fr, ok)
		}
		if again, _ := FrameOf(f); again != fr {
			t.Errorf("%s: frame not deterministic", name)
		}
		var box image.Rectangle
		for _, run := range Points(f) {
			for _, p := range run {
				x, y := fr.project(p)
				box = box.Union(image.Rect(int(math.Floor(x)), int(math.Floor(y)), int(math.Ceil(x))+1, int(math.Ceil(y))+1))
			}
		}
		// Inside the area, and filling it in one direction (zoom rounded down by < 0.01: < 1 %).
		if !box.In(mapArea.Inset(-2)) || box.Dx() < mapArea.Dx()*97/100 && box.Dy() < mapArea.Dy()*97/100 {
			t.Errorf("%s: route box %v, area %v", name, box, mapArea)
		}
	}
	if _, ok := FrameOf(public(t, parse(t, "indoor-no-gps.fit"))); ok {
		t.Error("frame without positions")
	}
	// Degenerate routes: capped zoom, no error.
	for _, f := range []datafile.File{file([2]float64{50, 8}), file([2]float64{50, 8}, [2]float64{50.00001, 8.00001})} {
		if fr, ok := FrameOf(f); !ok || fr.Zoom > maxZoom || fr.Zoom < 14 {
			t.Errorf("degenerate route: %+v", fr)
		}
	}
	// One position moved: another key (parser seam, AC 43).
	a, _ := FrameOf(file([2]float64{50, 8}, [2]float64{50.01, 8.02}))
	b, _ := FrameOf(file([2]float64{50, 8}, [2]float64{50.01, 8.021}))
	if a.Key == b.Key {
		t.Error("moving a position kept the key")
	}
}

// TestFrameHidingOn (AC 2): a file whose ends widen the box gets another frame
// with hiding on, and that frame is fitted to the public positions only.
func TestFrameHidingOn(t *testing.T) {
	w := parse(t, "ride-30min.fit")
	cut, _ := workout.HideEnds(w)
	off, _ := FrameOf(public(t, w))
	on, _ := FrameOf(public(t, cut))
	if off.Key == on.Key || on.Zoom <= off.Zoom {
		t.Errorf("hiding on: %+v, off: %+v", on, off)
	}
}

func synthMap(seed uint64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0xf2, 0xef, 0xe9, 0xff}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(700, 0, 900, 630), image.NewUniform(color.RGBA{0xaa, 0xd3, 0xdf, 0xff}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(950, 100, 1150, 300), image.NewUniform(color.RGBA{0xc8, 0xe6, 0xc0, 0xff}), image.Point{}, draw.Src)
	r := rand.New(rand.NewPCG(seed, 1))
	var roads [][][2]float64
	for range 60 {
		roads = append(roads, [][2]float64{{r.Float64() * Height, r.Float64() * Width}, {r.Float64() * Height, r.Float64() * Width}})
	}
	stroke(img, roads, func(p [2]float64) (float64, float64) { return p[1], p[0] }, 5, color.RGBA{0xb0, 0xb0, 0xb0, 0xff})
	stroke(img, roads, func(p [2]float64) (float64, float64) { return p[1], p[0] }, 3, color.RGBA{0xff, 0xff, 0xff, 0xff})
	return img
}

func encode(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngChunk is one chunk with its CRC.
func pngChunk(typ string, data []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	b = append(b, typ...)
	b = append(b, data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}

// chunks lists the chunk types of a PNG.
func chunks(t *testing.T, b []byte) []string {
	t.Helper()
	var out []string
	for i := 8; i+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[i:]))
		out = append(out, string(b[i+4:i+8]))
		i += 12 + n
	}
	return out
}

// insert puts chunks right after IHDR.
func insert(b []byte, cs ...[]byte) []byte {
	out := append([]byte{}, b[:33]...)
	for _, c := range cs {
		out = append(out, c...)
	}
	return append(out, b[33:]...)
}

// rawPNG is a PNG with the given IHDR and one IDAT holding zlib(raw).
func rawPNG(w, h uint32, depth, colourType, interlace byte, raw []byte) []byte {
	ihdr := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, w), h)
	ihdr = append(ihdr, depth, colourType, 0, 0, interlace)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(raw)
	zw.Close()
	b := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", ihdr)...)
	b = append(b, pngChunk("IDAT", z.Bytes())...)
	return append(b, pngChunk("IEND", nil)...)
}

// interlaced is a grey 1200 × 630 Adam7 PNG.
func interlaced() []byte {
	var raw []byte
	for _, p := range [][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}} {
		x0, y0, dx, dy := p[0], p[1], p[2], p[3]
		pw, ph := (Width-x0+dx-1)/dx, (Height-y0+dy-1)/dy
		if pw == 0 || ph == 0 {
			continue
		}
		for range ph {
			raw = append(raw, 0)
			raw = append(raw, bytes.Repeat([]byte{0x80}, pw)...)
		}
	}
	return rawPNG(Width, Height, 8, 0, 1, raw)
}

// TestCheckMap (AC 39, 40; MC-5): crafted PNGs refused with a fixed reason,
// honest variants accepted and flattened to opaque RGB.
func TestCheckMap(t *testing.T) {
	good := encode(t, synthMap(1))
	var jp, gf bytes.Buffer
	jpeg.Encode(&jp, synthMap(1), nil)
	gif.Encode(&gf, image.NewPaletted(image.Rect(0, 0, Width, Height), color.Palette{color.Black, color.White}), nil)
	row := append([]byte{0}, make([]byte, Width*3)...)
	full := bytes.Repeat(row, Height)
	badCRC := bytes.Clone(good)
	badCRC[len(badCRC)-13]++ // last byte of the IDAT CRC
	ancillaryCRC := insert(good, pngChunk("tEXt", []byte("Comment\x00x")))
	ancillaryCRC[33+8+9]++
	for _, c := range []struct {
		name   string
		data   []byte
		reason string
	}{
		{"empty", nil, ReasonFormat},
		{"text", []byte("hello, this is not a PNG at all"), ReasonFormat},
		{"jpeg", jp.Bytes(), ReasonFormat},
		{"gif", gf.Bytes(), ReasonFormat},
		{"signature only", good[:8], ReasonFormat},
		{"1201 × 630", encode(t, image.NewGray(image.Rect(0, 0, 1201, 630))), ReasonDimensions},
		{"1200 × 631", encode(t, image.NewGray(image.Rect(0, 0, 1200, 631))), ReasonDimensions},
		{"65535 × 65535", rawPNG(65535, 65535, 16, 6, 0, nil), ReasonDimensions},
		{"truncated IDAT", good[:len(good)/2], ReasonDecode},
		{"no IEND", good[:len(good)-12], ReasonDecode},
		{"bad CRC", badCRC, ReasonDecode},
		{"bad ancillary CRC", ancillaryCRC, ReasonDecode},
		{"zlib stream far larger than the image", rawPNG(Width, Height, 8, 2, 0, append(full, make([]byte, 50<<20)...)), ReasonDecode},
		{"too little pixel data", rawPNG(Width, Height, 8, 2, 0, full[:len(full)/2]), ReasonDecode},
		{"trailing data", append(bytes.Clone(good), "trailing"...), ReasonDecode},
		{"chunk after IEND", append(bytes.Clone(good), pngChunk("tEXt", []byte("a\x00b"))...), ReasonDecode},
	} {
		img, reason := CheckMap(c.data)
		if reason != c.reason || img != nil {
			t.Errorf("%s: %q, want %q", c.name, reason, c.reason)
		}
	}
	rgba64 := image.NewRGBA64(image.Rect(0, 0, Width, Height))
	draw.Draw(rgba64, rgba64.Bounds(), synthMap(2), image.Point{}, draw.Src)
	pal := image.NewPaletted(image.Rect(0, 0, Width, Height), color.Palette{color.RGBA{1, 2, 3, 255}, color.RGBA{200, 100, 50, 255}})
	translucent := image.NewNRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(translucent, translucent.Bounds(), image.NewUniform(color.NRGBA{0, 0, 0, 0}), image.Point{}, draw.Src)
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"rgb", good},
		{"16-bit", encode(t, rgba64)},
		{"paletted", encode(t, pal)},
		{"grey", encode(t, image.NewGray(image.Rect(0, 0, Width, Height)))},
		{"interlaced", interlaced()},
		{"transparent", encode(t, translucent)},
		{"text chunks", insert(good, pngChunk("tEXt", []byte("Comment\x00SECRET-TEXT")), pngChunk("iTXt", []byte("Note\x00\x00\x00\x00\x00SECRET-ITXT")), pngChunk("eXIf", []byte("MM\x00*SECRET-EXIF")))},
	} {
		img, reason := CheckMap(c.data)
		if reason != "" || img.Bounds() != image.Rect(0, 0, Width, Height) {
			t.Fatalf("%s refused: %q", c.name, reason)
		}
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0xff {
				t.Fatalf("%s: not opaque", c.name)
			}
		}
		stored, err := EncodeMap(img)
		if err != nil || bytes.Contains(stored, []byte("SECRET")) { // private copy; the served card is checked below
			t.Errorf("%s: stored bytes carry the upload's chunks", c.name)
		}
	}
	if img, _ := CheckMap(encode(t, translucent)); img.RGBAAt(5, 5) != background {
		t.Error("transparent pixels not flattened onto the background")
	}
}

// TestCheckMapBombHeader (MC-5): a 65535 × 65535 header is refused before
// any pixel memory is allocated.
func TestCheckMapBombHeader(t *testing.T) {
	data := rawPNG(65535, 65535, 16, 6, 0, make([]byte, 1<<10))
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range 10 {
		if _, reason := CheckMap(data); reason != ReasonDimensions {
			t.Fatal(reason)
		}
	}
	runtime.ReadMemStats(&after)
	if n := after.TotalAlloc - before.TotalAlloc; n > 1<<20 {
		t.Errorf("%d bytes allocated for 10 refused headers", n)
	}
}

func mapCard(t *testing.T, f datafile.File, title string, basemap *image.RGBA) ([]byte, error) {
	t.Helper()
	fr, ok := FrameOf(f)
	if !ok {
		t.Fatal("no frame")
	}
	return RenderOnMap(f, page.Build(f, page.Settings{Title: title, PublicID: "x"}), fr, encode(t, basemap))
}

// TestRenderOnMap (MC-6): 1200 × 630 paletted PNG under 300 KB with only
// critical chunks, deterministic, the route inside the map area, the panel
// and the attribution box drawn over the basemap.
func TestRenderOnMap(t *testing.T) {
	for _, name := range []string{"ride-30min.fit", "run-30min.fit", "ride-12h.fit", "triathlon.fit"} {
		f := public(t, parse(t, name))
		b, err := mapCard(t, f, "Morning loop", synthMap(3))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		again, _ := mapCard(t, f, "Morning loop", synthMap(3))
		if !bytes.Equal(b, again) {
			t.Errorf("%s: not deterministic", name)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil || img.Bounds() != image.Rect(0, 0, Width, Height) || len(b) > MaxBytes {
			t.Fatalf("%s: %v, %d bytes", name, err, len(b))
		}
		p, ok := img.(*image.Paletted)
		if !ok || len(p.Palette) > 256 {
			t.Errorf("%s: not paletted", name)
		}
		for _, c := range chunks(t, b) {
			if c != "IHDR" && c != "PLTE" && c != "tRNS" && c != "IDAT" && c != "IEND" {
				t.Errorf("%s: chunk %s", name, c)
			}
		}
		n, box := routePixels(img)
		if n < 1000 || !box.In(mapArea.Inset(-routeWidth)) {
			t.Errorf("%s: %d route pixels in %v", name, n, box)
		}
		if !near(img.At(panelWidth/2, 600), background) || !near(img.At(Width-20, 590), background) {
			t.Errorf("%s: no panel or attribution box", name)
		}
	}
}

// TestRenderOnMapHidden (AC 2): with hiding on, every route pixel lies at a
// public position (within the stroke); the hidden ends draw nothing.
func TestRenderOnMapHidden(t *testing.T) {
	cut, _ := workout.HideEnds(parse(t, "ride-30min.fit"))
	on := public(t, cut)
	fr, _ := FrameOf(on)
	b, err := mapCard(t, on, "", synthMap(4))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(b))
	var pts [][2]float64
	for _, run := range Points(on) {
		for _, p := range run {
			x, y := fr.project(p)
			pts = append(pts, [2]float64{x, y})
		}
	}
	bad := 0
	for y := range Height {
		for x := panelWidth; x < Width; x++ {
			if !near(img.At(x, y), routeInk) {
				continue
			}
			nearest := math.Inf(1)
			for _, p := range pts {
				nearest = min(nearest, math.Hypot(p[0]-float64(x), p[1]-float64(y)))
			}
			if nearest > routeWidth {
				bad++
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d route pixels away from every public position", bad)
	}
}

// TestRenderOnMapNoise: a basemap that cannot compress under 300 KB is
// refused (the build then draws the server-made card).
func TestRenderOnMapNoise(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	r := rand.New(rand.NewPCG(7, 7))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.Uint32())
	}
	if _, err := mapCard(t, public(t, parse(t, "ride-30min.fit")), "", img); err != ErrTooLarge {
		t.Errorf("noise: %v", err)
	}
}

// FuzzCheckMap: no panic and bounded work for any bytes through the check,
// the stored encoding and the composition.
func FuzzCheckMap(f *testing.F) {
	f.Add(encode(f, image.NewGray(image.Rect(0, 0, Width, Height))))
	f.Add(interlaced())
	f.Add(rawPNG(65535, 65535, 16, 6, 0, nil))
	f.Add(rawPNG(Width, Height, 8, 2, 0, []byte{0, 1, 2}))
	f.Add([]byte("not a png"))
	route := file([2]float64{50, 8}, [2]float64{50.01, 8.02})
	vm := page.Build(route, page.Settings{PublicID: "x"})
	fr, _ := FrameOf(route)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxMapBytes {
			return
		}
		img, reason := CheckMap(data)
		if reason != "" {
			return
		}
		stored, err := EncodeMap(img)
		if err != nil {
			t.Fatal(err)
		}
		if out, err := RenderOnMap(route, vm, fr, stored); err == nil && (bytes.Equal(out, data) || len(out) > MaxBytes) {
			t.Fatal("served bytes equal the upload or exceed the limit")
		}
	})
}

// TestSampleMapCards writes map review cards over a synthetic basemap when
// WX_CARDS names a directory (MC-6: the user reviews the look once).
func TestSampleMapCards(t *testing.T) {
	dir := os.Getenv("WX_CARDS")
	if dir == "" {
		t.Skip("set WX_CARDS to a directory to write the review cards")
	}
	for _, c := range []struct{ name, file, title string }{
		{"map-ride", "ride-30min.fit", "Morning loop"},
		{"map-run", "run-30min.fit", "Evening run along the river with friends"},
		{"map-triathlon", "triathlon.fit", ""},
	} {
		b, err := mapCard(t, public(t, parse(t, c.file)), c.title, synthMap(5))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, c.name+".png"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
