package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFill(t *testing.T) {
	src := "<footer><a href=\"/about\">About</a> " + Marker + "</footer>"
	if got, err := Fill("x", src, true); err != nil || got != `<footer><a href="/about">About</a> `+Link+"</footer>" {
		t.Errorf("on: %q %v", got, err)
	}
	if got, err := Fill("x", src, false); err != nil || got != `<footer><a href="/about">About</a> </footer>` {
		t.Errorf("off: %q %v", got, err)
	}
	for _, bad := range []string{"<footer></footer>", src + Marker} {
		if _, err := Fill("x.html", bad, true); err == nil || !strings.Contains(err.Error(), "x.html") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestWrite(t *testing.T) {
	web, data := t.TempDir(), t.TempDir()
	for _, n := range append(Shells, Impressum) {
		os.WriteFile(filepath.Join(web, n), []byte(n+":"+Marker), 0o600)
	}
	check := func(on bool) {
		t.Helper()
		if err := Write(data, web, on); err != nil {
			t.Fatal(err)
		}
		for _, n := range Shells {
			b, err := os.ReadFile(filepath.Join(data, "site", n))
			fi, _ := os.Stat(filepath.Join(data, "site", n))
			if err != nil || strings.Contains(string(b), Marker) || strings.Contains(string(b), Link) != on || fi.Mode().Perm() != fileMode {
				t.Errorf("%s (on=%v): %q %v", n, on, b, err)
			}
		}
		_, err := os.Stat(filepath.Join(data, "site", Impressum))
		if (err == nil) != on {
			t.Errorf("impressum shell present = %v, want %v", err == nil, on)
		}
		es, _ := os.ReadDir(filepath.Join(data, "site"))
		for _, e := range es {
			if strings.HasPrefix(e.Name(), ".") {
				t.Errorf("temporary left: %s", e.Name())
			}
		}
	}
	check(true)
	check(false)
	check(true)
	// A shell without its marker or a missing one fails the start (ADR-043 item 1).
	os.WriteFile(filepath.Join(web, "generic.html"), []byte("no marker"), 0o600)
	if err := Write(data, web, false); err == nil || !strings.Contains(err.Error(), "generic.html") {
		t.Errorf("no marker: %v", err)
	}
	os.Remove(filepath.Join(web, "generic.html"))
	if err := Write(data, web, false); err == nil {
		t.Error("missing shell accepted")
	}
}
