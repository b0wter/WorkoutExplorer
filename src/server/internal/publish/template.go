package publish

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"

	"workout-explorer/internal/page"
	"workout-explorer/internal/shell"
)

// LoadTemplate reads <webDir>/index.html, the deployable page template
// (F-15 P-10, ADR-026 item 1), parses it with missingkey=error and executes
// it once with a filled view model, so a template that names a field the
// view model lacks fails at start, not at the first publish. The footer
// marker is filled first (html/template drops comments): the Impressum link
// when impressum, else nothing (F-38 IM-7).
func LoadTemplate(webDir string, impressum bool) (*template.Template, error) {
	src, err := os.ReadFile(filepath.Join(webDir, "index.html"))
	if err != nil {
		return nil, fmt.Errorf("page template: %w", err)
	}
	filled, err := shell.Fill("page template", string(src), impressum)
	if err != nil {
		return nil, err
	}
	t, err := template.New("index.html").Option("missingkey=error").Parse(filled)
	if err != nil {
		return nil, fmt.Errorf("page template: %w", err)
	}
	if err := t.Execute(io.Discard, probe); err != nil {
		return nil, fmt.Errorf("page template: %w", err)
	}
	return t, nil
}

// probe has every group present, so that every action of the template runs.
var probe = page.ViewModel{
	Page: page.Group{Title: "t", ViewPath: "/w/x", DataURL: "/w/x/data.0.bin"},
	Header: page.Header{Title: "t", SportName: "Ride", SportIcon: "cycling", Start: "s", Description: "d",
		ExpiresAt: time.Unix(0, 0).UTC().Format(time.RFC3339), ExpiresText: "e"},
	Summary:     []page.SummaryItem{{Key: "k", Label: "l", Value: "1", Unit: "u", Max: "2"}},
	Device:      "d",
	Route:       page.Route{Map: "yes", NoLocation: "yes", Hidden: "yes"},
	ChartsClass: "charts-1 cells-2",
	Laps: &page.Laps{Columns: []page.Column{{Key: "number", Label: "#"}}, Rows: [][]string{{"1"}},
		Sections: []page.LapsSection{{Name: "Swim", Span: "1", Rows: [][]string{{"1"}}}}},
}
