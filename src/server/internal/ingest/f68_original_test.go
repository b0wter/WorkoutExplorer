package ingest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestStoredOriginalKeepsDeveloperFields: UP-9 is unchanged (F-68 OQ-331):
// the stored original keeps every developer field, name and application ID
// byte for byte. The corpus files carry no serial number or name, so the
// strip changes nothing and the original equals the upload.
func TestStoredOriginalKeepsDeveloperFields(t *testing.T) {
	for _, name := range []string{"stryd-power.fit", "stryd-and-native-power.fit", "stryd-two-apps.fit", "stryd-chained.fit", "developer-hostile.fit"} {
		upload, err := os.ReadFile(filepath.Join(corpusDir, "synthetic", name))
		if err != nil {
			t.Fatal(err)
		}
		acc, err := testParser.Accept(context.Background(), upload)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(acc.Original, upload) {
			t.Errorf("%s: the stored original differs from the upload (%d bytes, was %d)", name, len(acc.Original), len(upload))
		}
		for _, m := range []string{"Form Power", "Leg Spring Stiffness", "Home address", "SECRET-APP-ID-47", "Elm Street 5, Springfield"} {
			if bytes.Contains(upload, []byte(m)) && !bytes.Contains(acc.Original, []byte(m)) {
				t.Errorf("%s: %q lost from the stored original", name, m)
			}
		}
	}
}
