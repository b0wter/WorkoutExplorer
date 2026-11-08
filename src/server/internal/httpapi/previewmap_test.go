package httpapi

import (
	"context"
	"io"
	"strings"
	"testing"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/preview"
)

// F-36 MC-4: the two owner endpoints map the service's answers; the token
// check and every rule are in lifecycle.

func (f *fakeOwner) PreviewFrame(_ context.Context, _, tok string) (lifecycle.MapFrame, error) {
	f.calls, f.token = append(f.calls, "frame"), tok
	if f.err != nil {
		return lifecycle.MapFrame{}, f.err
	}
	if f.noFrame {
		return lifecycle.MapFrame{}, nil
	}
	return lifecycle.MapFrame{Frame: preview.Frame{Lon: 8.5, Lat: 50.25, Zoom: 12.34, Key: "0123456789abcdef"}, OK: true}, nil
}

func (f *fakeOwner) PutPreviewMap(_ context.Context, _, tok, key string, body io.Reader) error {
	f.calls, f.token, f.key = append(f.calls, "map"), tok, key
	b, err := io.ReadAll(io.LimitReader(body, preview.MaxMapBytes+1))
	f.read = len(b)
	if f.err == nil && err != nil {
		return &lifecycle.MapRefusedError{Reason: "read"}
	}
	if f.err == nil && len(b) > preview.MaxMapBytes {
		return &lifecycle.MapRefusedError{Reason: "size"}
	}
	return f.err
}

func TestPreviewMapEndpoints(t *testing.T) {
	auth := "Bearer " + strings.Repeat("t", 22)
	base := "/api/workouts/" + pid + "/preview-map"
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})

	rec := ownerDo(h, "GET", base, auth, "")
	if rec.Code != 200 || rec.Body.String() != `{"frame":{"lon":8.5,"lat":50.25,"zoom":12.34},"key":"0123456789abcdef","current":false}` ||
		rec.Header().Get("Cache-Control") != "no-store" || o.token != strings.Repeat("t", 22) {
		t.Errorf("get: %d %s", rec.Code, rec.Body)
	}
	o.noFrame = true
	if rec := ownerDo(h, "GET", base, auth, ""); rec.Code != 200 || rec.Body.String() != `{"frame":null}` {
		t.Errorf("no frame: %d %s", rec.Code, rec.Body)
	}

	if rec := ownerDo(h, "PUT", base+"/0123456789abcdef", auth, "PNG"); rec.Code != 204 || rec.Body.Len() != 0 ||
		rec.Header().Get("Cache-Control") != "no-store" || o.key != "0123456789abcdef" {
		t.Errorf("put: %d %s", rec.Code, rec.Body)
	}
	for _, c := range []struct {
		err  error
		code int
		body string
	}{
		{lifecycle.ErrNotFound, 404, wantNotFound},
		{lifecycle.ErrMapBusy, 503, `{"error":"busy","message":"Another preview image is being processed. Please try again."}`},
		{lifecycle.ErrMapStale, 409, `{"error":"stale","message":"The preview frame has changed."}`},
		{&lifecycle.MapRefusedError{Reason: "decode"}, 400, `{"error":"invalid_image","message":"The preview image was refused."}`},
		{&lifecycle.ServerError{Stage: "x", Err: io.EOF}, 500, ""},
	} {
		o.err = c.err
		rec := ownerDo(h, "PUT", base+"/0123456789abcdef", auth, "PNG")
		if rec.Code != c.code || c.body != "" && rec.Body.String() != c.body || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%v: %d %s", c.err, rec.Code, rec.Body)
		}
	}
	o.err = nil
	// Over the cap: 413, and the handler stops the transfer one byte after it.
	rec = ownerDo(h, "PUT", base+"/0123456789abcdef", auth, strings.Repeat("x", preview.MaxMapBytes+100))
	if rec.Code != 413 || rec.Body.String() != `{"error":"too_large","message":"The preview image was refused."}` || o.read != preview.MaxMapBytes+1 {
		t.Errorf("over the cap: %d %s, read %d", rec.Code, rec.Body, o.read)
	}
}
