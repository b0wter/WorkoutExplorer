package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/places"
	"workout-explorer/internal/workout"
)

// F-47 (UP-6, TR-2, TR-7, TR-8): the trimmed part never reaches a public
// file. Synthetic files only.

const markerHR = 197 // occurs only in the part that is trimmed away

// trimTrack is 900 s: a 600 s run from 50 N, 8 E northwards at 1.4 m/s and a
// 300 s drive 100 km away (heart rate markerHR, altitude 3000 m, 20 m/s). With
// driveFirst the drive comes first.
func trimTrack(driveFirst bool) workout.Workout {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	recs := make([]workout.RawRecord, 900)
	for i := range recs {
		drive := i >= 600
		if driveFirst {
			drive = i < 300
		}
		r := workout.RawRecord{At: t0.Add(time.Duration(i) * time.Second), Lat: math.NaN(), Lon: math.NaN(),
			Cadence: math.NaN(), Power: math.NaN(), Temperature: math.NaN()}
		switch {
		case drive:
			r.Lat, r.Lon = 51+float64(i)/10000, 9
			r.Altitude, r.Distance, r.Speed, r.HeartRate = 3000+float64(i), 50000+float64(i)*20, 20, markerHR
		default:
			j := float64(i)
			if driveFirst {
				j -= 300
			}
			r.Lat, r.Lon = 50+j*1.4/111195.08, 8
			r.Altitude, r.Distance, r.Speed, r.HeartRate = 200+float64(int(j)%20), j*1.4, 1.4, 120+float64(int(j)%20)
		}
		recs[i] = r
	}
	return workout.Normalise(workout.Recording{FileType: "activity", Records: recs})
}

func trimIndex(t *testing.T) *places.Index {
	ix, err := places.Parse("Runtown\t50\t8\nDrivetown\t51.05\t9\n")
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func instant(w workout.Workout, i int) *time.Time {
	t := w.StartTime.Add(time.Duration(w.Samples.Offset[i]) * time.Second)
	return &t
}

// TestTrimKeepsTheTrimmedPartOutOfEveryPublicFile (AC 3, 7, 10, 38): end and
// start trims, hiding off and on.
func TestTrimKeepsTheTrimmedPartOutOfEveryPublicFile(t *testing.T) {
	for _, driveFirst := range []bool{false, true} {
		for _, hide := range []bool{false, true} {
			w := trimTrack(driveFirst)
			d := draft()
			d.HideEnds = hide
			runFrom, runTo := 0, 599
			if driveFirst {
				runFrom, runTo = 300, 899
				d.TrimStartAt = instant(w, runFrom)
			} else {
				d.TrimEndAt = instant(w, runTo)
			}
			fx := newFixtureWith(t, []byte("trk"), fakeParser{"trk": w})
			fx.a.d.Places = trimIndex(t)
			fx.change(t, d, published(d))
			out := fx.read(t) // also: .br equals the data file, the preview is the render of it

			if hr, _ := stream(out.file, "heart_rate"); slices.ContainsFunc(hr.Ints, func(v int64) bool { return v == markerHR }) {
				t.Errorf("drive %v hide %v: marker heart rate in the data file", driveFirst, hide)
			}
			if lat, ok := stream(out.file, "latitude"); ok {
				for k, v := range lat.Ints {
					if lat.Valid[k] && v > 50_100_000 {
						t.Fatalf("a position of the drive is public: %d", v)
					}
				}
			}
			if alt, _ := stream(out.file, "altitude"); slices.ContainsFunc(alt.Ints, func(v int64) bool { return v >= 20000 }) {
				t.Error("drive altitude in the data file")
			}
			if out.file.Meta.StartTime != w.StartTime.Add(time.Duration(runFrom)*time.Second).Format(time.RFC3339) {
				t.Errorf("start time %s", out.file.Meta.StartTime)
			}
			if n := len(out.file.Streams[0].Ints); n != 600 {
				t.Errorf("%d samples", n)
			}
			if dist, err := out.file.Meta.Summary.Distance.Float64(); err != nil || dist > 840 {
				t.Errorf("distance %v is not the run's (%v)", dist, err)
			}
			entries, _ := os.ReadDir(fx.folder())
			for _, e := range entries {
				b, _ := os.ReadFile(filepath.Join(fx.folder(), e.Name()))
				if bytes.Contains(b, []byte("Drivetown")) {
					t.Errorf("%s names the trimmed part's town", e.Name())
				}
			}
			if !bytes.Contains(out.page, []byte("Runtown")) {
				t.Error("the page lost the run's town")
			}

			// Owner view = public: summary bytes, suggestion, ranges shifted into draft.bin.
			v, err := fx.a.TrimmedView(context.Background(), d)
			if err != nil {
				t.Fatal(err)
			}
			var pub json.RawMessage
			full, _ := json.Marshal(out.file.Meta.Summary)
			pub = full
			if !bytes.Equal(v.Trimmed.Summary, pub) {
				t.Errorf("owner summary %s\npublic %s", v.Trimmed.Summary, pub)
			}
			m := ogTitle.FindSubmatch(out.page)
			if html.UnescapeString(string(m[1])) != v.Suggested {
				t.Errorf("suggestion %q, public %q", v.Suggested, m[1])
			}
			if hide && (len(v.Ranges) != 2 || v.Ranges[0][0] != runFrom || v.Ranges[1][1] != runTo) {
				t.Errorf("ranges %v", v.Ranges)
			}
		}
	}
}

// TestTrimChangesRepublishRemoveFirst (AC 43, 44): every trim change on a
// published workout, widening included, removes first; removing the trim
// gives the files of a never-trimmed build.
func TestTrimChangesRepublishRemoveFirst(t *testing.T) {
	w := trimTrack(false)
	fx := newFixtureWith(t, []byte("trk"), fakeParser{"trk": w})
	fx.a.step = func(s string) error { fx.steps = append(fx.steps, s); return nil }
	d := draft()
	pub := published(d)
	fx.change(t, d, pub)
	untrimmed := fx.read(t)

	narrow := pub
	narrow.TrimEndAt = instant(w, 400)
	wide := pub
	wide.TrimEndAt = instant(w, 599)
	prev, prevOut := pub, untrimmed
	for _, next := range []lifecycle.Record{narrow, wide, pub} {
		fx.steps = nil
		fx.change(t, prev, next)
		if len(fx.steps) < 2 || fx.steps[0] != "remove index.html" || fx.steps[1] != "remove folder" {
			t.Errorf("steps %v: a trim change must remove first", fx.steps)
		}
		out := fx.read(t)
		for _, n := range out.names {
			if n != "index.html" && slices.Contains(prevOut.names, n) && next.TrimEndAt != nil && prev.TrimEndAt != nil && n == dataName(t, prevOut.page) {
				t.Errorf("old data file %s still on disk", n)
			}
		}
		prev, prevOut = next, out
	}
	if !slices.Equal(prevOut.names, untrimmed.names) || !bytes.Equal(prevOut.bin, untrimmed.bin) || !bytes.Equal(prevOut.page, untrimmed.page) {
		t.Error("removing the trim does not restore the untrimmed build")
	}
	// The allow-list never takes the trim columns.
	for _, r := range []lifecycle.Record{narrow, wide} {
		a, b := pub, r
		if !HidesMore(a, b) || !HidesMore(b, a) {
			t.Error("a trim change is not remove-first")
		}
	}
}

// TestTrimResolveAndMinimum (AC 41, TR-3, TR-4): snapping to samples, null
// for an untrimmed end, and the refusal below a minute.
func TestTrimResolveAndMinimum(t *testing.T) {
	w := trimTrack(false)
	fx := newFixtureWith(t, []byte("trk"), fakeParser{"trk": w})
	ctx := context.Background()
	s, e, err := fx.a.ResolveTrim(ctx, draft(), instant(w, 0), instant(w, 899))
	if err != nil || s != nil || e != nil {
		t.Errorf("the first and last sample must store null: %v %v %v", s, e, err)
	}
	s, e, err = fx.a.ResolveTrim(ctx, draft(), instant(w, 10), instant(w, 70))
	if err != nil || !s.Equal(*instant(w, 10)) || !e.Equal(*instant(w, 70)) {
		t.Errorf("60 s: %v %v %v", s, e, err)
	}
	if _, _, err = fx.a.ResolveTrim(ctx, draft(), instant(w, 10), instant(w, 69)); err != lifecycle.ErrTrimTooShort {
		t.Errorf("59 s: %v", err)
	}
}

// peakHeap runs f and returns the highest heap in use a 200 µs sampler saw.
func peakHeap(f func()) uint64 {
	defer debug.SetGCPercent(debug.SetGCPercent(5)) // heap in use is then close to what is live
	runtime.GC()
	var peak uint64
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			peak = max(peak, m.HeapAlloc)
			select {
			case <-stop:
				return
			case <-time.After(200 * time.Microsecond):
			}
		}
	}()
	f()
	close(stop)
	<-done
	return peak
}

// TestTrimMemoryStopRule (ADR-047, TR-10): the peak heap of a trimmed build
// of a 12 h file does not exceed the untrimmed build's.
func TestTrimMemoryStopRule(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	n := 43200
	recs := make([]workout.RawRecord, n)
	for i := range recs {
		recs[i] = workout.RawRecord{At: t0.Add(time.Duration(i) * time.Second), Lat: 50 + float64(i)*1e-5, Lon: 8, Altitude: 200 + float64(i%50),
			Distance: float64(i) * 3, Speed: 3, HeartRate: 140, Cadence: 80, Power: 200, Temperature: 20}
	}
	w := workout.Normalise(workout.Recording{FileType: "activity", Records: recs})
	fx := newFixtureWith(t, []byte("big"), fakeParser{"big": w})
	d := draft()
	trimmedRec := d
	trimmedRec.TrimEndAt = instant(w, n-61) // the least that can be cut: the largest trimmed copy
	var plain, cut uint64
	for range 3 { // best of three, the sampler is noisy
		p := peakHeap(func() { _, _, _, _ = fx.a.front(context.Background(), d) })
		c := peakHeap(func() { _, _, _, _ = fx.a.front(context.Background(), trimmedRec) })
		if plain == 0 || p < plain {
			plain = p
		}
		if cut == 0 || c < cut {
			cut = c
		}
	}
	t.Logf("peak heap: untrimmed %d MB, trimmed %d MB", plain>>20, cut>>20)
	if cut > plain+plain/10 { // 10 % for sampler noise; the trim adds a few streams of 8 bytes per sample
		t.Errorf("trimmed peak %d MB above untrimmed %d MB (TR-10 stop rule)", cut>>20, plain>>20)
	}
}
