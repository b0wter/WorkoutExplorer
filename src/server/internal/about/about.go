// Package about serves the About page (F-29 AB-3, AB-4). The page is the
// frontend's template, finished by the build (the attribution list) and
// by serve at start (per-deployment values); this package does the second part
// and answers GET and HEAD /about from memory.
//
// Template contract (comment markers; the frontend template carries each):
//
//	<!--wx:attributions-->   build time: tools/attributions replaces it (once) with <!--wx:generated--> and the list
//	<!--wx:operator-->       serve: the operator file as <h2>, <p> and <ul> (exactly once; AB-8), or the placeholder paragraph
//	<!--wx:version-->        serve: the binary's version, HTML-escaped (any number of times, also inside the list)
//	<!--wx:pushover-->...<!--/wx:pushover-->  exactly one pair; kept (markers dropped) only when notifications are on
package about

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Marker strings; tools/attributions has the build-time pair.
const (
	MarkerList    = "<!--wx:attributions-->"
	MarkerDone    = "<!--wx:generated-->"
	markerOp      = "<!--wx:operator-->"
	markerVersion = "<!--wx:version-->"
	markerPushOn  = "<!--wx:pushover-->"
	markerPushOff = "<!--/wx:pushover-->"
)

// Placeholder is the operator block while the operator file is missing (decision 25, AB-8).
const Placeholder = "<p>[OPERATOR DETAILS: who runs this site, contact, hosting, web server logs, backups, legal basis, complaints]</p>\n"

// MaxOperatorFile is the size limit of the operator file (AB-8).
const MaxOperatorFile = 16 << 10

// Values are the per-deployment facts filled at start. Operator is the
// rendered operator block (RenderOperator); empty shows the placeholder.
type Values struct {
	Operator, Version string
	Notifications     bool
}

// ParseOperator checks the operator file's content and renders it (AB-8):
// "## Text" is an <h2>, consecutive "- Text" lines one <ul>, other non-empty
// lines join with a space into a <p>, a blank line ends a block. Every
// character is HTML-escaped; nothing else is markup.
func ParseOperator(b []byte) (string, error) {
	if len(b) > MaxOperatorFile {
		return "", errors.New("longer than 16 KiB")
	}
	if !utf8.Valid(b) {
		return "", errors.New("not valid UTF-8")
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if strings.IndexFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return "", errors.New("contains a control character")
	}
	var out strings.Builder
	var para []string
	var list []string
	flush := func() {
		if len(para) > 0 {
			out.WriteString("<p>" + html.EscapeString(strings.Join(para, " ")) + "</p>\n")
			para = nil
		}
		if len(list) > 0 {
			out.WriteString("<ul>\n")
			for _, l := range list {
				out.WriteString("<li>" + html.EscapeString(l) + "</li>\n")
			}
			out.WriteString("</ul>\n")
			list = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "## "):
			flush()
			out.WriteString("<h2>" + html.EscapeString(strings.TrimSpace(line[3:])) + "</h2>\n")
		case strings.HasPrefix(line, "- "):
			if len(para) > 0 {
				flush()
			}
			list = append(list, strings.TrimSpace(line[2:]))
		default:
			if len(list) > 0 {
				flush()
			}
			para = append(para, line)
		}
	}
	flush()
	if out.Len() == 0 {
		return "", errors.New("has no content")
	}
	return out.String(), nil
}

// Load reads <webDir>/about.html and fills it. It fails when the file is
// missing, has no generated list (a build that skipped the generator), or
// lacks a required marker.
func Load(webDir string, v Values) ([]byte, error) {
	src, err := os.ReadFile(filepath.Join(webDir, "about.html"))
	if err != nil {
		return nil, fmt.Errorf("about page: %w", err)
	}
	return fill(src, v)
}

func fill(src []byte, v Values) ([]byte, error) {
	s := string(src)
	switch {
	case strings.Contains(s, MarkerList) || !strings.Contains(s, MarkerDone):
		return nil, errors.New("about page: the attribution list is not generated (run make build)")
	case strings.Count(s, markerOp) != 1:
		return nil, errors.New("about page: needs exactly one operator marker")
	}
	on, off := strings.Index(s, markerPushOn), strings.Index(s, markerPushOff)
	if on < 0 || off < on || strings.Count(s, markerPushOn) != 1 || strings.Count(s, markerPushOff) != 1 {
		return nil, errors.New("about page: the pushover block is missing or malformed")
	}
	// The block first, so nothing inserted later is ever scanned again.
	inner := s[on+len(markerPushOn) : off]
	if !v.Notifications {
		inner = ""
	}
	s = s[:on] + inner + s[off+len(markerPushOff):]
	op := v.Operator
	if op == "" {
		op = Placeholder
	}
	// One pass: an inserted value that contains a marker is not expanded.
	return []byte(strings.NewReplacer(MarkerDone, "", markerOp, op, markerVersion, html.EscapeString(v.Version)).Replace(s)), nil
}

// Handler answers GET and HEAD (register as "GET /about") with the page.
func Handler(page []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(page))
	})
}
