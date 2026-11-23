package publish

import "testing"

// F-66 RA-7: the setting is written in place in either direction, and a
// toggle changes nothing else.
func TestHidesMoreForRemoveAutoLaps(t *testing.T) {
	a, b := published(draft()), published(draft())
	b.RemoveAutoLaps = true
	if HidesMore(a, b) || HidesMore(b, a) {
		t.Error("the setting is rebuilt remove-first")
	}
}
