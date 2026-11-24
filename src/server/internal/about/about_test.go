package about

import (
	"os"
	"strings"
	"testing"
)

// TestNtfyPushSentence (QA-70-r2-3): the FCM/Google/Apple sentence is in the
// rendered About page only with ntfy.
func TestNtfyPushSentence(t *testing.T) {
	for _, c := range []struct {
		n    Notify
		want bool
	}{{Notify{Kind: "ntfy", Host: "ntfy.example"}, true}, {Notify{Kind: "pushover"}, false}, {Notify{Kind: "callback"}, false}, {Notify{}, false}} {
		src, _ := os.ReadFile("../../../web/about.html")
		got, err := fill([]byte(strings.Replace(string(src), "<!--wx:attributions-->", MarkerDone, 1)), Values{Notify: c.n})
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(got), `data-t="ab_ntfyPush"`); has != c.want {
			t.Errorf("kind %q: sentence present %v, want %v", c.n.Kind, has, c.want)
		}
	}
}

func TestParseOperator(t *testing.T) {
	in := "## Who <runs> it\r\nJane\nDoe\n- a & b\n- c\n\nText <!--wx:version--> {{ x }}\n- late\n"
	want := "<h2>Who &lt;runs&gt; it</h2>\n<p>Jane Doe</p>\n<ul>\n<li>a &amp; b</li>\n<li>c</li>\n</ul>\n<p>Text &lt;!--wx:version--&gt; {{ x }}</p>\n<ul>\n<li>late</li>\n</ul>\n"
	if got, err := ParseOperator([]byte(in)); err != nil || got != want {
		t.Errorf("got %q, %v", got, err)
	}
	for name, b := range map[string]string{"empty": "\n \n", "ctl": "a\x07", "big": strings.Repeat("a", MaxOperatorFile+1)} {
		if _, err := ParseOperator([]byte(b)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// Nothing inserted is scanned again: a marker in the operator block stays text.
func TestFillNoRescan(t *testing.T) {
	src := "<!--wx:operator--><!--wx:pushover-->P<!--/wx:pushover--><!--wx:ntfy-->NTFY <!--wx:notify-host--><!--/wx:ntfy--><!--wx:callback-->CB<!--/wx:callback--><!--wx:shortcut-->S<!--/wx:shortcut--><!--wx:updates--><!--/wx:updates--><!--wx:generated--><!--wx:impressum-->"
	got, err := fill([]byte(src), Values{Operator: "<p>&lt;!--wx:version--&gt;</p>", Version: "v"})
	if err != nil || strings.Contains(string(got), "P") || strings.Contains(string(got), "S") || !strings.Contains(string(got), "&lt;!--wx:version--&gt;") {
		t.Errorf("%q %v", got, err)
	}
}

func TestParseOperatorBOM(t *testing.T) {
	if got, err := ParseOperator([]byte("\xef\xbb\xbf## Who\nJane\n")); err != nil || got != "<h2>Who</h2>\n<p>Jane</p>\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

// TestFillShortcut (F-34): the block only with a link; the link escaped in
// the href and the text; a link holding a marker is not expanded.
func TestFillShortcut(t *testing.T) {
	src := `<!--wx:operator--><!--wx:pushover--><!--/wx:pushover--><!--wx:ntfy-->NTFY <!--wx:notify-host--><!--/wx:ntfy--><!--wx:callback-->CB<!--/wx:callback--><!--wx:shortcut--><a href="<!--wx:shortcut-url-->"><!--wx:shortcut-url--></a><!--/wx:shortcut--><!--wx:updates--><!--/wx:updates--><!--wx:generated--><!--wx:impressum-->`
	link := `https://www.icloud.com/shortcuts/a?b=1&c="><script><!--wx:version-->`
	esc := `https://www.icloud.com/shortcuts/a?b=1&amp;c=&#34;&gt;&lt;script&gt;&lt;!--wx:version--&gt;`
	got, err := fill([]byte(src), Values{Shortcut: link, Version: "v"})
	if want := Placeholder + `<a href="` + esc + `">` + esc + `</a>`; err != nil || string(got) != want {
		t.Errorf("on: %q %v", got, err)
	}
	if got, err := fill([]byte(src), Values{}); err != nil || string(got) != Placeholder {
		t.Errorf("off: %q %v", got, err)
	}
	for _, bad := range []string{"<!--wx:shortcut-->", "<!--/wx:shortcut-->"} {
		if _, err := fill([]byte(strings.Replace(src, bad, "", 1)), Values{}); err == nil {
			t.Errorf("without %s accepted", bad)
		}
	}
}

// TestFillImpressum (F-38 IM-7): the link only when set; the marker is required once.
func TestFillImpressum(t *testing.T) {
	src := "<!--wx:operator--><!--wx:pushover--><!--/wx:pushover--><!--wx:ntfy-->NTFY <!--wx:notify-host--><!--/wx:ntfy--><!--wx:callback-->CB<!--/wx:callback--><!--wx:shortcut--><!--/wx:shortcut--><!--wx:updates--><!--/wx:updates--><!--wx:generated--><footer><!--wx:impressum--></footer>"
	if got, err := fill([]byte(src), Values{Impressum: true}); err != nil || !strings.HasSuffix(string(got), `<footer><a href="/impressum">Impressum</a></footer>`) {
		t.Errorf("on: %q %v", got, err)
	}
	if got, err := fill([]byte(src), Values{}); err != nil || !strings.HasSuffix(string(got), "<footer></footer>") {
		t.Errorf("off: %q %v", got, err)
	}
	if _, err := fill([]byte(strings.Replace(src, "<!--wx:impressum-->", "", 1)), Values{}); err == nil {
		t.Error("without the marker accepted")
	}
}

// TestFillUpdates (F-52 AU-12, PT-9a): the block only when automatic updates
// are on; the pair is required once.
func TestFillUpdates(t *testing.T) {
	src := "<!--wx:operator--><!--wx:pushover--><!--/wx:pushover--><!--wx:ntfy-->NTFY <!--wx:notify-host--><!--/wx:ntfy--><!--wx:callback-->CB<!--/wx:callback--><!--wx:shortcut--><!--/wx:shortcut--><!--wx:updates-->U<!--/wx:updates--><!--wx:generated--><!--wx:impressum-->"
	if got, err := fill([]byte(src), Values{Updates: true}); err != nil || string(got) != Placeholder+"U" {
		t.Errorf("on: %q %v", got, err)
	}
	if got, err := fill([]byte(src), Values{}); err != nil || string(got) != Placeholder {
		t.Errorf("off: %q %v", got, err)
	}
	for _, bad := range []string{"<!--wx:updates-->", "<!--/wx:updates-->"} {
		if _, err := fill([]byte(strings.Replace(src, bad, "", 1)), Values{Updates: true}); err == nil {
			t.Errorf("without %s accepted", bad)
		}
	}
}
