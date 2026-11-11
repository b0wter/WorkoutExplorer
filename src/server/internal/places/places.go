// Package places finds the nearest town of 15,000 or more people to one
// point, offline, from the GeoNames table embedded in the binary (F-45 ST-2,
// ADR-045). No network code, and nothing here is ever logged (I-5).
package places

import (
	"bufio"
	_ "embed"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"workout-explorer/internal/workout"
)

// Radius is the farthest a town may be from the point, in metres, inclusive
// (UP-5: more than 25 km gives no town).
const Radius = 25_000

//go:embed places.tsv
var table string

type town struct {
	name     string
	lat, lon float64
}

// Index is an immutable list of towns; the zero value and nil find nothing.
type Index struct{ towns []town }

// Parse reads the TSV (name, latitude, longitude per line).
func Parse(tsv string) (*Index, error) {
	var ix Index
	sc := bufio.NewScanner(strings.NewReader(tsv))
	for n := 1; sc.Scan(); n++ {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("places: line %d: %d fields", n, len(f))
		}
		lat, err1 := strconv.ParseFloat(f[1], 64)
		lon, err2 := strconv.ParseFloat(f[2], 64)
		if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, fmt.Errorf("places: line %d: bad coordinates", n)
		}
		ix.towns = append(ix.towns, town{f[0], lat, lon})
	}
	return &ix, sc.Err()
}

var (
	once sync.Once
	def  *Index
)

// Default is the embedded table, built once per process; nil if it does not
// parse (a defect make ci catches).
func Default() *Index {
	once.Do(func() {
		var err error
		if def, err = Parse(table); err != nil {
			slog.Error("place table unusable, suggested titles fall back to sport and date", "err", err) // Parse errors name a line number only
		}
	})
	return def
}

// Nearest is the town nearest to the point within Radius; ties go to the
// earlier row. ok is false when none is near or the index is empty.
// shortcut: linear scan (about 32,000 haversines), add a grid if a lookup measures over 10 ms.
func (ix *Index) Nearest(lat, lon float64) (name string, ok bool) {
	return ix.within(lat, lon, Radius)
}

func (ix *Index) within(lat, lon, radius float64) (name string, ok bool) {
	if ix == nil {
		return "", false
	}
	best := radius
	for _, t := range ix.towns {
		if d := workout.Haversine(lat, lon, t.lat, t.lon); d <= best && (!ok || d < best) {
			best, name, ok = d, t.name, true
		}
	}
	return name, ok
}
