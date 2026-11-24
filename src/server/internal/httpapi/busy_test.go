package httpapi

import (
	"strings"
	"testing"

	"workout-explorer/internal/lifecycle"
)

// TestOwnerWriteBusy (ADR-061 item 4): a second write of one workout is a 503
// with the code the edit page has a text for.
func TestOwnerWriteBusy(t *testing.T) {
	auth := "Bearer " + strings.Repeat("t", 22)
	o := &fakeOwner{err: lifecycle.ErrBusy}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	want := `{"error":"busy","message":"Another change to this workout is being saved. Please try again."}`
	for _, c := range []struct{ method, path, body string }{
		{"PATCH", "/api/workouts/" + pid, `{"title":"x"}`},
		{"POST", "/api/workouts/" + pid + "/publish", ""},
	} {
		if rec := ownerDo(h, c.method, c.path, auth, c.body); rec.Code != 503 || rec.Body.String() != want {
			t.Errorf("%s: %d %s", c.method, rec.Code, rec.Body)
		}
	}
}
