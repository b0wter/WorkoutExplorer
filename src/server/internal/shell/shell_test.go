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
		os.WriteFile(filepath.Join(web, n), []byte(n+":"+Marker+ShortcutOn+ShortcutOff), 0o600)
	}
	check := func(on bool) {
		t.Helper()
		if err := Write(data, web, on, ""); err != nil {
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
	if err := Write(data, web, false, ""); err == nil || !strings.Contains(err.Error(), "generic.html") {
		t.Errorf("no marker: %v", err)
	}
	os.Remove(filepath.Join(web, "generic.html"))
	if err := Write(data, web, false, ""); err == nil {
		t.Error("missing shell accepted")
	}
}

// TestShortcutHint (F-63): the upload page source unchanged but for the
// block when the link is unset; with a link, the hint with the link escaped
// in attribute and text; a missing or broken block fails the start.
func TestShortcutHint(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "web", upload))
	if err != nil {
		t.Fatal(err)
	}
	write := func(page, link string) (string, error) {
		web, data := t.TempDir(), t.TempDir()
		for _, n := range Shells {
			os.WriteFile(filepath.Join(web, n), []byte(Marker), 0o600)
		}
		os.WriteFile(filepath.Join(web, upload), []byte(page), 0o600)
		if err := Write(data, web, false, link); err != nil {
			return "", err
		}
		b, err := os.ReadFile(filepath.Join(data, "site", upload))
		return string(b), err
	}
	i, j := strings.Index(string(src), ShortcutOn), strings.Index(string(src), ShortcutOff)
	if i < 0 || j < i {
		t.Fatal("upload.html has no shortcut block")
	}
	today := strings.Replace(string(src[:i])+string(src[j+len(ShortcutOff):]), Marker, "", 1)
	if got, err := write(string(src), ""); err != nil || got != today || strings.Contains(got, "shortcut") {
		t.Errorf("unset: %v\n%s", err, got)
	}
	got, err := write(string(src), `https://www.icloud.com/shortcuts/x?a=1&b="><script>`)
	if err != nil || !strings.Contains(got, `<a href="https://www.icloud.com/shortcuts/x?a=1&amp;b=&#34;&gt;&lt;script&gt;" data-p="link"`) ||
		strings.Contains(got, `"><script>`) || strings.Contains(got, "wx:shortcut") || !strings.Contains(got, `href="/about#shortcut"`) {
		t.Errorf("set: %v\n%s", err, got)
	}
	for _, bad := range []string{Marker, Marker + ShortcutOff + ShortcutOn, Marker + ShortcutOn + ShortcutOn + ShortcutOff} {
		if _, err := write(bad, "https://x.example/"); err == nil || !strings.Contains(err.Error(), upload) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
