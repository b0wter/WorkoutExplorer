package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F-36 end to end through newApp and regenerate (architect/F-36-preview-image-with-map-context.md,
// Verification). Synthetic files and a synthetic basemap only.

// noEgress fails the test on any request through http.DefaultTransport (AC 45).
type noEgress struct{ t *testing.T }

func (n noEgress) RoundTrip(r *http.Request) (*http.Response, error) {
	n.t.Errorf("outgoing request to %s", r.URL.Host)
	return nil, errors.New("no egress")
}

func forbidEgress(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = noEgress{t}
	t.Cleanup(func() { http.DefaultTransport = old })
}

// upMap is a synthetic basemap as a browser would send it, with text,
// iTXt and eXIf chunks that must never reach the served file.
func upMap(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0xe8, 0xe4, 0xd8, 0xff}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(800, 0, 900, 630), image.NewUniform(color.RGBA{0xaa, 0xd3, 0xdf, 0xff}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 300, 1200, 306), image.NewUniform(color.RGBA{0xff, 0xff, 0xff, 0xff}), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	out := append([]byte{}, b[:33]...)
	for _, c := range [][2]string{{"tEXt", "Comment\x00SECRET-TEXT"}, {"iTXt", "Note\x00\x00\x00\x00\x00SECRET-ITXT"}, {"eXIf", "MM\x00*SECRET-EXIF"}} {
		ch := binary.BigEndian.AppendUint32(nil, uint32(len(c[1])))
		ch = append(append(ch, c[0]...), c[1]...)
		out = binary.BigEndian.AppendUint32(append(out, ch...), crc32.ChecksumIEEE(ch[4:]))
	}
	return append(out, b[33:]...)
}

type frameReply struct {
	Frame   *struct{ Lon, Lat, Zoom float64 } `json:"frame"`
	Key     string                            `json:"key"`
	Current bool                              `json:"current"`
}

func (a *app) frame(pub, token string) frameReply {
	a.t.Helper()
	rec := a.owner("GET", "/api/workouts/"+pub+"/preview-map", token, "")
	var f frameReply
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &f) != nil {
		a.t.Fatalf("frame: %d %s", rec.Code, rec.Body)
	}
	return f
}

func (a *app) putMap(pub, token, key string, body []byte) int {
	a.t.Helper()
	return a.owner("PUT", "/api/workouts/"+pub+"/preview-map/"+key, token, string(body)).Code
}

// served is the preview the page links.
func (a *app) served(pub string) []byte {
	a.t.Helper()
	b, err := os.ReadFile(filepath.Join(a.dir, "public", pub, previewName(a.t, a.page(pub))))
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}

func paletted(t *testing.T, b []byte) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := img.(*image.Paletted)
	return ok
}

func (a *app) maps(id string) []string {
	a.t.Helper()
	m, _ := filepath.Glob(filepath.Join(a.dir, "private", id, "preview-map.*"))
	return m
}

// TestPreviewMapEndToEnd (AC 1, 2, 40, 42, 44, 45): draft PUT, publish uses
// the map; hiding on drops it at once and a new PUT brings a map back in
// place; a title change keeps it; a stale key is 409; delete leaves nothing.
func TestPreviewMapEndToEnd(t *testing.T) {
	forbidEgress(t)
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := internalID(a.app)
	offset := a.st.Used() - diskUsage(t, filepath.Join(dir, "private")) - diskUsage(t, filepath.Join(dir, "public"))
	counted := func(step string) {
		t.Helper()
		if got := a.st.Used() - diskUsage(t, filepath.Join(dir, "private")) - diskUsage(t, filepath.Join(dir, "public")); got != offset {
			t.Errorf("%s: storage total off by %d", step, got-offset)
		}
	}
	up := upMap(t)

	f := a.frame(pub, token)
	if f.Frame == nil || len(f.Key) != 16 || f.Current {
		t.Fatalf("draft frame %+v", f)
	}
	if code := a.putMap(pub, token, f.Key, up); code != 204 {
		t.Fatalf("draft put: %d", code)
	}
	if a.publicFiles(pub) != nil || len(a.maps(id)) != 1 || !a.frame(pub, token).Current {
		t.Fatal("draft put: public files, or no stored map")
	}
	counted("draft put")

	// Publish picks the map up (AC 1, 40).
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	card := a.served(pub)
	if !paletted(t, card) || len(card) > 300_000 || bytes.Equal(card, up) || bytes.Contains(card, []byte("SECRET")) {
		t.Fatal("published card is not the map card, or carries upload bytes")
	}
	counted("publish")

	// Title change: map kept, text redrawn (AC 42).
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"title":"Renamed"}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	renamed := a.served(pub)
	if !paletted(t, renamed) || bytes.Equal(renamed, card) {
		t.Error("title change: map card not redrawn")
	}

	// Hiding on (AC 2, 42): F-23's card in the same build; a new frame.
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"hide_ends":true}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if paletted(t, a.served(pub)) {
		t.Fatal("hiding on: the map for the hiding-off frame is still served")
	}
	g := a.frame(pub, token)
	if g.Key == f.Key || g.Current || *g.Frame == *f.Frame {
		t.Fatalf("hiding on: frame %+v, before %+v", g, f)
	}
	// A late PUT from a stale tab (MC-4): 409, nothing changed.
	before := a.maps(id)
	if code := a.putMap(pub, token, f.Key, up); code != 409 || !slicesEqual(a.maps(id), before) || paletted(t, a.served(pub)) {
		t.Errorf("stale put: %d", code)
	}
	// The background job's PUT: rebuilt in place, the old map removed.
	if code := a.putMap(pub, token, g.Key, up); code != 204 || len(a.maps(id)) != 1 || !paletted(t, a.served(pub)) {
		t.Errorf("new put: %d, maps %v", code, a.maps(id))
	}
	counted("new put")

	// Delete (AC 44): nothing left; a PUT afterwards creates no folder.
	if rec := a.owner("DELETE", "/api/workouts/"+pub, token, ""); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if code := a.putMap(pub, token, g.Key, up); code != 404 {
		t.Errorf("put after delete: %d", code)
	}
	if left := dataLeft(t, dir, id, pub); len(left) > 0 {
		t.Errorf("left after delete: %v", left)
	}
	counted("delete")
	if strings.Contains(a.logs.String(), f.Key) || strings.Contains(a.logs.String(), g.Key) {
		t.Error("a frame key was logged")
	}
}

func slicesEqual(a, b []string) bool { return strings.Join(a, "\n") == strings.Join(b, "\n") }

// TestPreviewMapAccess (AC 41, 39): without a valid token, with a query
// string or a malformed key the not-found answer, nothing stored; crafted
// bodies refused with the card and the map unchanged.
func TestPreviewMapAccess(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	id := internalID(a)
	key := a.frame(pub, token).Key
	up := upMap(t)
	for _, c := range []struct{ name, method, path, token string }{
		{"get, no token", "GET", "/preview-map", ""},
		{"get, wrong token", "GET", "/preview-map", strings.Repeat("A", 22)},
		{"get, malformed token", "GET", "/preview-map", "short"},
		{"get, query string", "GET", "/preview-map?token=" + token, token},
		{"put, no token", "PUT", "/preview-map/" + key, ""},
		{"put, wrong token", "PUT", "/preview-map/" + key, strings.Repeat("A", 22)},
		{"put, malformed token", "PUT", "/preview-map/" + key, "short"},
		{"put, query string", "PUT", "/preview-map/" + key + "?x", token},
		{"put, key upper case", "PUT", "/preview-map/" + strings.ToUpper(key), token},
		{"put, key short", "PUT", "/preview-map/" + key[:15], token},
		{"put, key long", "PUT", "/preview-map/" + key + "0", token},
		{"put, key path", "PUT", "/preview-map/..%2f" + key[:12], token},
	} {
		rec := a.owner(c.method, "/api/workouts/"+pub+c.path, c.token, string(up))
		if rec.Code != 404 || rec.Body.String() != ownerNotFound {
			t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
		}
	}
	if len(a.maps(id)) != 0 {
		t.Fatal("stored without a valid token or key")
	}

	// Publish with a map, then crafted bodies: the card and the map stay.
	if a.putMap(pub, token, key, up) != 204 || a.owner("POST", "/api/workouts/"+pub+"/publish", token, "").Code != 200 {
		t.Fatal("setup")
	}
	card, stored := a.served(pub), a.privateFile(id, filepath.Base(a.maps(id)[0]))
	var small bytes.Buffer
	png.Encode(&small, image.NewGray(image.Rect(0, 0, 1201, 630)))
	for _, c := range []struct {
		name string
		body []byte
		code int
	}{
		{"over the cap", make([]byte, 4_000_001), 413},
		{"empty", nil, 400},
		{"text", []byte("hello"), 400},
		{"1201 × 630", small.Bytes(), 400},
		{"truncated", up[:len(up)/2], 400},
		{"trailing data", append(bytes.Clone(up), "x"...), 400},
	} {
		if code := a.putMap(pub, token, key, c.body); code != c.code {
			t.Errorf("%s: %d", c.name, code)
		}
		if !bytes.Equal(a.served(pub), card) || !bytes.Equal(a.privateFile(id, filepath.Base(a.maps(id)[0])), stored) || len(a.maps(id)) != 1 {
			t.Errorf("%s: card or map changed", c.name)
		}
	}
	for _, reason := range []string{"size", "format", "dimensions", "decode"} {
		if !strings.Contains(a.logs.String(), `"msg":"preview map refused","id":"`+id+`","reason":"`+reason+`"`) {
			t.Errorf("no log line for %s", reason)
		}
	}
}

// TestPreviewMapNoFrame (AC 38): indoor, and a route hidden entirely, get no
// frame; a PUT there is stale.
func TestPreviewMapNoFrame(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("indoor-no-gps.fit")
	rec := a.owner("GET", "/api/workouts/"+pub+"/preview-map", token, "")
	if rec.Code != 200 || rec.Body.String() != `{"frame":null}` {
		t.Errorf("indoor: %d %s", rec.Code, rec.Body)
	}
	if code := a.putMap(pub, token, "0123456789abcdef", upMap(t)); code != 409 {
		t.Errorf("indoor put: %d", code)
	}
}

// TestRegeneratePreviewMap (AC 43): regenerate keeps a map whose key still
// matches, and drops one whose frame changed (hiding set behind serve's back).
func TestRegeneratePreviewMap(t *testing.T) {
	forbidEgress(t)
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token := a.published("ride-30min.fit")
	key := a.frame(pub, token).Key
	if a.putMap(pub, token, key, upMap(t)) != 204 {
		t.Fatal("put")
	}
	card := a.served(pub)
	runRegen(t, context.Background(), dir, testWeb, nil).want(t, 0, "1", "0", "0", "0")
	if !bytes.Equal(a.served(pub), card) {
		t.Error("regenerate dropped a matching map")
	}
	a.sql("UPDATE workouts SET hide_ends = 1")
	runRegen(t, context.Background(), dir, testWeb, nil).want(t, 0, "1", "0", "0", "0")
	if paletted(t, a.served(pub)) {
		t.Error("regenerate kept a map for another frame")
	}
}
