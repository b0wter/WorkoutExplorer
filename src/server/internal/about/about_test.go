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
	src := "<!--wx:operator--><!--wx:pushover-->P<!--/wx:pushover--><!--wx:generated-->"
	got, err := fill([]byte(src), Values{Operator: "<p>&lt;!--wx:version--&gt;</p>", Version: "v"})
	if err != nil || strings.Contains(string(got), "P") || !strings.Contains(string(got), "&lt;!--wx:version--&gt;") {
		t.Errorf("%q %v", got, err)
	}
}

func TestParseOperatorBOM(t *testing.T) {
	if got, err := ParseOperator([]byte("\xef\xbb\xbf## Who\nJane\n")); err != nil || got != "<h2>Who</h2>\n<p>Jane</p>\n" {
		t.Errorf("got %q, %v", got, err)
	}
}
