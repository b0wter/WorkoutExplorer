// Package preview renders the Open Graph image of a published workout
// (F-23 PC-1 to PC-5, ADR-033): the route of the decoded public data file on
// a plain background, plus the sport icon, the card's name and values from
// the page view model. Pure: no I/O, no clock, no map iteration, so the bytes are
// deterministic. The route comes from the data file's positions only, so
// hidden ends (F-16) cannot appear in it (I-3).
package preview

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
)

// Size of the image (SH-5).
const (
	Width  = 1200
	Height = 630
)

// NameRunes is N of PC-4 (OQ-91): the name is cut to this many code points plus "…".
const NameRunes = 40

// Layout (PC-5): text stays inside the central 1200 × 600 band.
const (
	margin      = 64
	iconSize    = 56
	iconGap     = 20
	nameSize    = 48
	nameY       = 124
	labelSize   = 26
	valueSize   = 48
	unitSize    = 28
	rowHeight   = 124
	routeWidth  = 6
	minSpanM    = 200 // a route smaller than this is drawn at this span
	minStepPx   = 1.0 // points closer than this to the last kept one are skipped
	circleSides = 16
)

// routeBox is where the route is fitted, right of the value column.
var routeBox = image.Rect(560, 168, Width-margin, 566)

var (
	background = color.RGBA{0xf7, 0xf6, 0xf3, 0xff}
	ink        = color.RGBA{0x1c, 0x1c, 0x1e, 0xff}
	muted      = color.RGBA{0x6e, 0x6e, 0x73, 0xff}
	routeInk   = color.RGBA{0xc2, 0x18, 0x7a, 0xff} // the page's light --route
)

var regular, bold = mustParse(goregular.TTF), mustParse(gobold.TTF)

func mustParse(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err) // embedded constant
	}
	return f
}

// Render returns the PNG of the workout's card: f is the decoded public data
// file, vm the page view model built from it.
func Render(f datafile.File, vm page.ViewModel) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	// Values in a grid: two columns left of the route, or three across
	// without positions (indoor, fully hidden; no hint).
	cols, colWidth, top := 2, 240, 228
	if !drawRoute(img, f) {
		cols, colWidth, top = 3, 376, 300
	}
	drawCard(img, f, vm, nameSize, Width-margin, cols, colWidth, top)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawCard draws the icon, the name (in a face of size nameSz, ending before
// x = nameEnd) and the values in cols columns from baseline top (PC-3, PC-4).
func drawCard(img *image.RGBA, f datafile.File, vm page.ViewModel, nameSz float64, nameEnd, cols, colWidth, top int) {
	nameFace := face(bold, nameSz)
	labelFace := face(regular, labelSize)
	valueFace := face(bold, valueSize)
	unitFace := face(regular, unitSize)
	defer nameFace.Close()
	defer labelFace.Close()
	defer valueFace.Close()
	defer unitFace.Close()

	icon(img, vm.Header.SportIcon, margin, nameY-iconSize+8)
	nameX := margin + iconSize + iconGap
	name := Name(vm.Card.Title, page.DefaultTitle(f), nameFace, nameEnd-nameX)
	text(img, nameFace, ink, nameX, nameY, name)
	for i, it := range page.CardItems(f, vm) {
		value(img, labelFace, valueFace, unitFace, margin+i%cols*colWidth, top+i/cols*rowHeight, it)
	}
}

func face(f *opentype.Font, size float64) font.Face {
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err) // constant options
	}
	return fc
}

func text(img draw.Image, f font.Face, c color.Color, x, y int, s string) {
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// value draws one label above its value and unit; labelY is the label's baseline.
func value(img draw.Image, label, val, unit font.Face, x, labelY int, it page.SummaryItem) {
	text(img, label, muted, x, labelY, it.Label)
	d := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: val, Dot: fixed.P(x, labelY+54)}
	d.DrawString(it.Value)
	if it.Unit != "" {
		d.Face, d.Src = unit, image.NewUniform(muted)
		d.DrawString(" " + it.Unit)
	}
}

// Name is the name as drawn (PC-4, OQ-92): characters the bold Go font cannot
// draw are dropped, white space becomes one space; nothing left gives
// fallback. Then it is cut to NameRunes code points plus "…" and further
// until it fits maxWidth pixels in f.
func Name(title, fallback string, f font.Face, maxWidth int) string {
	runes := drawable(title)
	if len(runes) == 0 {
		runes = drawable(fallback)
	}
	cut := len(runes) > NameRunes
	if cut {
		runes = runes[:NameRunes]
	}
	for {
		s := string(runes)
		if cut {
			s = strings.TrimRightFunc(s, unicode.IsSpace) + "…"
		}
		if len(runes) == 0 || font.MeasureString(f, s) <= fixed.I(maxWidth) {
			return s
		}
		runes, cut = runes[:len(runes)-1], true
	}
}

func drawable(s string) []rune {
	var b sfnt.Buffer
	var out []rune
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = len(out) > 0
			continue
		}
		if g, err := bold.GlyphIndex(&b, r); err != nil || g == 0 || unicode.IsControl(r) {
			continue
		}
		if space {
			out = append(out, ' ')
			space = false
		}
		out = append(out, r)
	}
	return out
}

// Points returns the route as runs of consecutive valid positions of f, in
// degrees (lat, lon). A missing position breaks the run. No position stream,
// or none valid: nil.
func Points(f datafile.File) [][][2]float64 {
	var lat, lon []float64
	for _, s := range f.Streams {
		switch s.Descriptor.Name {
		case "latitude":
			lat = s.Values()
		case "longitude":
			lon = s.Values()
		}
	}
	if lat == nil || lon == nil {
		return nil
	}
	var runs [][][2]float64
	var cur [][2]float64
	for i := range min(len(lat), len(lon)) {
		if math.IsNaN(lat[i]) || math.IsNaN(lon[i]) {
			if len(cur) > 0 {
				runs = append(runs, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, [2]float64{lat[i], lon[i]})
	}
	if len(cur) > 0 {
		runs = append(runs, cur)
	}
	return runs
}

// drawRoute fits the route into routeBox and strokes it; false without a route.
func drawRoute(img *image.RGBA, f datafile.File) bool {
	runs := Points(f)
	if runs == nil {
		return false
	}
	minLat, maxLat, minLon, maxLon := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, run := range runs {
		for _, p := range run {
			minLat, maxLat = min(minLat, p[0]), max(maxLat, p[0])
			minLon, maxLon = min(minLon, p[1]), max(maxLon, p[1])
		}
	}
	// Equirectangular metres around the box centre: enough for one route.
	lat0, lon0 := (minLat+maxLat)/2, (minLon+maxLon)/2
	kx, ky := math.Cos(lat0*math.Pi/180)*111320, 110574.0
	w, h := (maxLon-minLon)*kx, (maxLat-minLat)*ky
	bw, bh := float64(routeBox.Dx()-routeWidth), float64(routeBox.Dy()-routeWidth)
	scale := min(bw, bh) / minSpanM
	if w > 0 {
		scale = min(scale, bw/w)
	}
	if h > 0 {
		scale = min(scale, bh/h)
	}
	cx, cy := float64(routeBox.Min.X+routeBox.Max.X)/2, float64(routeBox.Min.Y+routeBox.Max.Y)/2
	proj := func(p [2]float64) (float64, float64) {
		return cx + (p[1]-lon0)*kx*scale, cy - (p[0]-lat0)*ky*scale
	}
	stroke(img, runs, proj, routeWidth, routeInk)
	return true
}

// stroke draws the runs, projected to pixels by proj, as a line width wide.
func stroke(img *image.RGBA, runs [][][2]float64, proj func([2]float64) (float64, float64), width float64, c color.Color) {
	z := vector.NewRasterizer(Width, Height)
	r := width / 2
	for _, run := range runs {
		var px, py float64
		for i, p := range run {
			x, y := proj(p)
			if i > 0 && math.Hypot(x-px, y-py) < minStepPx && i < len(run)-1 {
				continue
			}
			if i > 0 {
				segment(z, px, py, x, y, r)
			}
			circle(z, x, y, r)
			px, py = x, y
		}
		if len(run) == 1 {
			x, y := proj(run[0])
			circle(z, x, y, 2*r)
		}
	}
	z.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{})
}

// segment and circle add closed shapes of the same (clockwise on screen)
// orientation: vector sums signed coverage, so opposite windings would cancel
// where the route crosses itself.
func segment(z *vector.Rasterizer, x0, y0, x1, y1, r float64) {
	l := math.Hypot(x1-x0, y1-y0)
	if l == 0 {
		return
	}
	nx, ny := -(y1-y0)/l*r, (x1-x0)/l*r
	z.MoveTo(float32(x0+nx), float32(y0+ny))
	z.LineTo(float32(x1+nx), float32(y1+ny))
	z.LineTo(float32(x1-nx), float32(y1-ny))
	z.LineTo(float32(x0-nx), float32(y0-ny))
	z.ClosePath()
}

func circle(z *vector.Rasterizer, x, y, r float64) {
	z.MoveTo(float32(x+r), float32(y))
	for i := 1; i < circleSides; i++ {
		a := -2 * math.Pi * float64(i) / circleSides
		z.LineTo(float32(x+r*math.Cos(a)), float32(y+r*math.Sin(a)))
	}
	z.ClosePath()
}

// icon draws the sport icon (page Header.SportIcon) in an iconSize square at
// (x, y): strokes on a 24-unit grid, the same shapes as the route.
func icon(img *image.RGBA, sport string, x, y int) {
	z := vector.NewRasterizer(iconSize, iconSize)
	k := float64(iconSize) / 24
	line := func(pts ...float64) { // a round-capped polyline 2 units wide
		for i := 0; i+1 < len(pts); i += 2 {
			if i > 0 {
				segment(z, pts[i-2]*k, pts[i-1]*k, pts[i]*k, pts[i+1]*k, k)
			}
			circle(z, pts[i]*k, pts[i+1]*k, k)
		}
	}
	ring := func(cx, cy, r float64) {
		var pts []float64
		for i := 0; i <= 24; i++ {
			a := 2 * math.Pi * float64(i) / 24
			pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
		}
		line(pts...)
	}
	head := func(cx, cy float64) { circle(z, cx*k, cy*k, 2.4*k) }
	switch sport {
	case "cycling":
		ring(5, 17, 4)
		ring(19, 17, 4)
		line(5, 17, 9, 10, 15, 10, 11, 17, 5, 17)
		line(15, 10, 19, 17)
		line(9, 10, 8, 7)
		line(6.5, 7, 10, 7)
		line(15, 10, 14, 6, 17, 6)
	case "running":
		head(15, 3.5)
		line(14, 8, 11, 14)
		line(8, 11, 10, 8.5, 14, 8, 16, 11, 19, 11)
		line(11, 14, 15, 17, 14, 22)
		line(11, 14, 9, 19, 4, 19)
	case "walking", "hiking":
		head(13, 3.5)
		line(12.5, 8, 11.5, 15)
		line(9, 13, 10, 9.5, 12.5, 8, 14.5, 11, 16, 13)
		line(11.5, 15, 9, 22)
		line(11.5, 15, 14, 18, 14, 22)
	case "swimming":
		head(18, 8)
		line(3, 13, 8, 9, 12, 11, 15, 13)
		line(8, 9, 11, 6, 14, 7)
		line(2, 18, 5, 16, 8, 18, 11, 16, 14, 18, 17, 16, 20, 18, 22, 17)
	default:
		ring(12, 14, 8)
		line(12, 14, 12, 9)
		line(10, 3, 14, 3)
		line(12, 3, 12, 6)
		line(18.5, 6.5, 19.5, 5.5)
	}
	z.Draw(img, image.Rect(x, y, x+iconSize, y+iconSize), image.NewUniform(ink), image.Point{})
}
