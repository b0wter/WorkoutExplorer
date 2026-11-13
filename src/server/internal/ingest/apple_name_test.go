package ingest

import "testing"

func TestAppleName(t *testing.T) {
	for in, want := range map[string]string{
		"Watch7,5":     "Apple Watch Ultra 2",
		"Watch6,10":    "Apple Watch SE (2nd generation)",
		"iPhone16,1":   "iPhone 15 Pro",
		"Watch99,1":    "Apple Watch", // newer than the table
		"iPhone99,9":   "iPhone",
		"Edge 530":     "Edge 530",
		"Watch7,5 Pro": "Watch7,5 Pro",
		"iPad13,1":     "iPad13,1",
		"":             "",
	} {
		if got := appleName(in); got != want {
			t.Errorf("appleName(%q) = %q, want %q", in, got, want)
		}
	}
}
