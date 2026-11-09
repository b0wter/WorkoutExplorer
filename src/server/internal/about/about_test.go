package about

import (
	"strings"
	"testing"
)

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
	src := "<!--wx:operator--><!--wx:pushover-->P<!--/wx:pushover--><!--wx:shortcut-->S<!--/wx:shortcut--><!--wx:generated-->"
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
	src := `<!--wx:operator--><!--wx:pushover--><!--/wx:pushover--><!--wx:shortcut--><a href="<!--wx:shortcut-url-->"><!--wx:shortcut-url--></a><!--/wx:shortcut--><!--wx:generated-->`
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
