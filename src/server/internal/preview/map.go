package preview

// The map variant of the card (F-36 MC-2, MC-5, MC-6; ADR-041): the browser
// renders only the basemap for a frame computed here from the public data
// file; this file checks the uploaded basemap at the trust boundary and draws
// the card over it. Pure, like Render. The frame and its key are never logged
// (I-5).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"slices"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
)

// MaxMapBytes caps an uploaded basemap (MC-5): above the largest PNG a canvas
// writes at 1200 × 630 (RGBA, stored deflate, about 3.05 MB).
const MaxMapBytes = 4_000_000

// MaxBytes is the size limit of a served card (F-23 PC-5).
const MaxBytes = 300_000

// Frame is the camera the browser renders the basemap with: Web Mercator as
// MapLibre draws it (512 px world tile, bearing 0, pitch 0, 1200 × 630,
// pixel ratio 1). Values are rounded, and Go projects with the same values.
type Frame struct {
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
	Zoom float64 `json:"zoom"`
	Key  string  `json:"-"` // MC-2: binds a stored basemap to this frame
}

// layoutVersion is part of the key: a change of the frame rule or the map
// layout must change every key, so old basemaps are no longer used.
const layoutVersion = 1

// Map layout: an opaque panel on the left, the route fitted into mapArea.
const (
	panelWidth   = 520
	mapNameSize  = 36
	maxZoom      = 16
	worldPx      = 512
	maxMercLat   = 85.051129
	earthCircumM = 40075016.686
	attribution  = "© OpenStreetMap, OpenFreeMap"
	attribSize   = 18
	casingWidth  = routeWidth + 4
)

var (
	mapArea = image.Rect(panelWidth+48, 48, Width-48, 556)
	casing  = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

func mercX(lon float64) float64 { return (lon + 180) / 360 }

func mercY(lat float64) float64 {
	lat = max(-maxMercLat, min(maxMercLat, lat))
	s := math.Sin(lat * math.Pi / 180)
	return 0.5 - math.Log((1+s)/(1-s))/(4*math.Pi)
}

func latOf(y float64) float64 { return math.Atan(math.Sinh(math.Pi*(1-2*y))) * 180 / math.Pi }

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	v = math.Round(v*p) / p
	if v == 0 {
		return 0 // no "-0" in the key or the JSON
	}
	return v
}

// FrameOf is the preview frame of a public data file (MC-2): the bounding box
// of its positions fitted into mapArea, at least F-23's minimum span, zoom
// capped. ok is false without positions (indoor, fully hidden).
//
// shortcut: a route across the antimeridian or near the poles gets a wrong
// frame (a route beside the roads, never a leak); handle it if one shows up.
func FrameOf(f datafile.File) (fr Frame, ok bool) {
	runs := Points(f)
	if runs == nil {
		return Frame{}, false
	}
	minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, run := range runs {
		for _, p := range run {
			x, y := mercX(p[1]), mercY(p[0])
			minX, maxX, minY, maxY = min(minX, x), max(maxX, x), min(minY, y), max(maxY, y)
		}
	}
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	span := minSpanM / (earthCircumM * math.Cos(latOf(cy)*math.Pi/180))
	w, h := max(maxX-minX, span), max(maxY-minY, span)
	z := math.Log2(min(float64(mapArea.Dx())/(w*worldPx), float64(mapArea.Dy())/(h*worldPx)))
	z = math.Floor(max(0, min(maxZoom, z))*100) / 100 // down, so the route stays inside
	// The camera centre is the canvas centre; shift it so the box centre
	// lands on the centre of mapArea.
	scale := worldPx * math.Exp2(z)
	ax, ay := float64(mapArea.Min.X+mapArea.Max.X)/2, float64(mapArea.Min.Y+mapArea.Max.Y)/2
	camX, camY := cx-(ax-Width/2)/scale, cy-(ay-Height/2)/scale
	fr = Frame{Lon: round(camX*360-180, 6), Lat: round(latOf(camY), 6), Zoom: z}
	sum := sha256.Sum256(fmt.Appendf(nil, "wx-map %d %.6f %.6f %.2f", layoutVersion, fr.Lon, fr.Lat, fr.Zoom))
	fr.Key = hex.EncodeToString(sum[:8])
	return fr, true
}

// project maps (lat, lon) to canvas pixels with the frame's rounded values.
func (fr Frame) project(p [2]float64) (float64, float64) {
	scale := worldPx * math.Exp2(fr.Zoom)
	return Width/2 + (mercX(p[1])-mercX(fr.Lon))*scale, Height/2 + (mercY(p[0])-mercY(fr.Lat))*scale
}

// ValidKey reports whether s has the form of a key: 16 lowercase hex.
func ValidKey(s string) bool {
	if len(s) != 16 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Reasons of a refused basemap (MC-5), logged as they are.
const (
	ReasonFormat     = "format"
	ReasonDimensions = "dimensions"
	ReasonDecode     = "decode"
)

// CheckMap is the trust boundary of MC-5 for bytes already read under
// MaxMapBytes: the header must say PNG of exactly Width × Height before any
// pixel is allocated; then the whole PNG must decode with nothing after
// IEND. Any colour model is flattened onto the card background to opaque
// RGB. reason is "" when accepted.
func CheckMap(data []byte) (img *image.RGBA, reason string) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ReasonFormat
	}
	if cfg.Width != Width || cfg.Height != Height {
		return nil, ReasonDimensions
	}
	r := bytes.NewReader(data)
	src, err := png.Decode(r) // bounded by the fixed size: at most 6 MB at 16 bits
	if err != nil || r.Len() != 0 {
		return nil, ReasonDecode
	}
	img = image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Over)
	return img, ""
}

// EncodeMap is the stored form of an accepted basemap: our own PNG encoding
// of its pixels, never the received bytes (MC-5).
func EncodeMap(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ErrTooLarge: the card over the map is above MaxBytes even at the smallest
// palette; the caller draws Render's card.
var ErrTooLarge = errors.New("preview: map card too large")

// RenderOnMap draws the card over a stored basemap (MC-6): the basemap on the
// whole canvas, an opaque panel on the left with the icon, name and values,
// the route of f with a light casing projected by fr, and the attribution.
// The output is a paletted PNG of at most 256 colours. Any error means the
// caller draws Render's card.
func RenderOnMap(f datafile.File, vm page.ViewModel, fr Frame, basemap []byte) ([]byte, error) {
	img, reason := CheckMap(basemap)
	if reason != "" {
		return nil, errors.New("preview: stored map: " + reason)
	}
	runs := Points(f)
	if runs == nil {
		return nil, errors.New("preview: no route")
	}
	stroke(img, runs, fr.project, casingWidth, casing)
	stroke(img, runs, fr.project, routeWidth, routeInk)
	draw.Draw(img, image.Rect(0, 0, panelWidth, Height), image.NewUniform(background), image.Point{}, draw.Src)
	drawCard(img, f, vm, mapNameSize, panelWidth-32, 2, 216, 228)

	af := face(regular, attribSize)
	defer af.Close()
	w := font.MeasureString(af, attribution).Ceil()
	const pad, right, bottom = 8, Width - 12, 606 // inside the central 1200 × 600 band
	box := image.Rect(right-w-2*pad, bottom-attribSize-2*pad, right, bottom)
	draw.Draw(img, box, image.NewUniform(background), image.Point{}, draw.Src)
	d := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: af, Dot: fixed.P(box.Min.X+pad, box.Max.Y-pad-4)}
	d.DrawString(attribution)

	for _, n := range []int{256, 128, 64} {
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, quantize(img, n)); err != nil {
			return nil, err
		}
		if buf.Len() <= MaxBytes {
			return buf.Bytes(), nil
		}
	}
	return nil, ErrTooLarge
}

// quantize reduces img to at most n opaque colours by median cut over a
// 15-bit colour histogram; deterministic (no map iteration, stable sorts).
func quantize(img *image.RGBA, n int) *image.Paletted {
	const buckets = 1 << 15
	bucket := func(r, g, b uint8) uint16 { return uint16(r>>3)<<10 | uint16(g>>3)<<5 | uint16(b>>3) }
	count := make([]uint32, buckets)
	sum := make([][3]uint64, buckets)
	pix := img.Pix
	for i := 0; i+3 < len(pix); i += 4 {
		k := bucket(pix[i], pix[i+1], pix[i+2])
		count[k]++
		sum[k][0] += uint64(pix[i])
		sum[k][1] += uint64(pix[i+1])
		sum[k][2] += uint64(pix[i+2])
	}
	var keys []uint16
	for k, c := range count {
		if c > 0 {
			keys = append(keys, uint16(k))
		}
	}
	channel := func(k uint16, c int) int { return int(k>>(10-5*c)) & 31 }
	// widest returns the channel with the largest range in box and the range.
	widest := func(box []uint16) (int, int) {
		best, rng := 0, -1
		for c := range 3 {
			lo, hi := 31, 0
			for _, k := range box {
				v := channel(k, c)
				lo, hi = min(lo, v), max(hi, v)
			}
			if hi-lo > rng {
				best, rng = c, hi-lo
			}
		}
		return best, rng
	}
	boxes := [][]uint16{keys}
	for len(boxes) < n {
		// Split the box with the most pixels × widest range.
		pick, score := -1, uint64(0)
		for i, b := range boxes {
			if len(b) < 2 {
				continue
			}
			_, rng := widest(b)
			var pop uint64
			for _, k := range b {
				pop += uint64(count[k])
			}
			if s := pop * uint64(rng); s > score {
				pick, score = i, s
			}
		}
		if pick < 0 {
			break
		}
		b := boxes[pick]
		c, _ := widest(b)
		slices.SortStableFunc(b, func(x, y uint16) int { return channel(x, c) - channel(y, c) })
		var total, acc uint64
		for _, k := range b {
			total += uint64(count[k])
		}
		cut := 1
		for i, k := range b[:len(b)-1] {
			acc += uint64(count[k])
			cut = i + 1
			if acc*2 >= total {
				break
			}
		}
		boxes[pick] = b[:cut]
		boxes = append(boxes, b[cut:])
	}
	pal := make(color.Palette, len(boxes))
	index := make([]uint8, buckets)
	for i, b := range boxes {
		var r, g, bl, c uint64
		for _, k := range b {
			index[k] = uint8(i)
			r, g, bl, c = r+sum[k][0], g+sum[k][1], bl+sum[k][2], c+uint64(count[k])
		}
		pal[i] = color.RGBA{uint8((r + c/2) / c), uint8((g + c/2) / c), uint8((bl + c/2) / c), 0xff}
	}
	out := image.NewPaletted(img.Bounds(), pal)
	for i, j := 0, 0; i+3 < len(pix); i, j = i+4, j+1 {
		out.Pix[j] = index[bucket(pix[i], pix[i+1], pix[i+2])]
	}
	return out
}
