package ingest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/corpustest"
	"workout-explorer/internal/workout"
)

// Tests added by F-08 v1.1 (QA round 1): sequence gate, sport hints, R-12 on
// real encoder output, SampleCount over the corpus.

// recordingOf runs the sequence walk and returns the raw recording.
func recordingOf(t *testing.T, data []byte) workout.Recording {
	t.Helper()
	lim := DefaultLimits.withDefaults()
	l := newListener(lim, nil)
	res := walk(context.Background(), data, l, lim)
	rec := l.rec
	rec.Integrity = res.integ
	return rec
}

func sportMesg(s typedef.Sport) proto.Message {
	return mesgdef.NewSport(nil).SetSport(s).SetSubSport(typedef.SubSportGeneric).ToMesg(nil)
}

func settingsFileID() proto.Message {
	id := fileID()
	for i := range id.Fields {
		if id.Fields[i].Num == 0 {
			id.Fields[i].Value = proto.Uint8(uint8(typedef.FileSettings))
		}
	}
	return id
}

// TestChainedFirstSequenceGate (QA-08-r1-4, D-15): only a file_id in the
// first sequence decides the file type and the device.
func TestChainedFirstSequenceGate(t *testing.T) {
	var noID []proto.Message
	for i := 0; i < 10; i++ {
		noID = append(noID, record(i, 100).ToMesg(nil))
	}
	first := encodeFIT(t, noID...)

	t.Run("first sequence without file_id, then settings", func(t *testing.T) {
		second := encodeFIT(t, settingsFileID(), record(100, 100).ToMesg(nil))
		w := mustParse(t, append(append([]byte(nil), first...), second...))
		if w.Integrity.Sequences != 2 || w.Integrity.SkippedSequences != 1 || len(w.Samples.Offset) != 10 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
		if w.FileType != "" || w.Device != (workout.Device{}) {
			t.Errorf("file type %q device %+v: a later sequence must not set them", w.FileType, w.Device)
		}
	})
	t.Run("first sequence without file_id, then an activity sequence with one", func(t *testing.T) {
		second := encodeFIT(t, fileID(), record(100, 100).ToMesg(nil))
		w := mustParse(t, append(append([]byte(nil), first...), second...))
		if w.Integrity.Sequences != 2 || w.Integrity.SkippedSequences != 0 || len(w.Samples.Offset) != 11 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
		if w.FileType != "" || w.Device != (workout.Device{}) {
			t.Errorf("file type %q device %+v: a later sequence must not set them", w.FileType, w.Device)
		}
	})
	t.Run("a settings first sequence still rejects", func(t *testing.T) {
		data := encodeFIT(t, settingsFileID(), record(0, 100).ToMesg(nil))
		if ie := parseErr(t, data); ie.Kind != KindUnsupportedType {
			t.Errorf("kind = %s", ie.Kind)
		}
	})
}

// TestSequenceStarts (R-12 input): the index of each later sequence's first
// record, repeated for a skipped sequence.
func TestSequenceStarts(t *testing.T) {
	seq := func(first, n int) []byte {
		msgs := []proto.Message{fileID()}
		for i := 0; i < n; i++ {
			msgs = append(msgs, record(first+i, 100).ToMesg(nil))
		}
		return encodeFIT(t, msgs...)
	}
	skipped := encodeFIT(t, settingsFileID(), record(500, 100).ToMesg(nil), record(501, 100).ToMesg(nil))
	data := bytes.Join([][]byte{seq(0, 10), skipped, seq(100, 5), seq(200, 3)}, nil)
	rec := recordingOf(t, data)
	if want := []int{10, 10, 15}; !reflect.DeepEqual(rec.SequenceStarts, want) {
		t.Errorf("SequenceStarts = %v, want %v", rec.SequenceStarts, want)
	}
	if got := recordingOf(t, seq(0, 4)).SequenceStarts; len(got) != 0 {
		t.Errorf("one sequence: SequenceStarts = %v", got)
	}
}

// TestListenerSport: sport messages become hints at the next record of the
// same sequence; lap.sport is read (D-12).
func TestListenerSport(t *testing.T) {
	r := func(s int) proto.Message { return record(s, 100).ToMesg(nil) }

	t.Run("a sport message before the first record gets that record's time", func(t *testing.T) {
		rec := recordingOf(t, encodeFIT(t, fileID(), sportMesg(typedef.SportRunning), r(5), r(6)))
		if want := []workout.RawSport{{At: at(5), Sport: "running"}}; !reflect.DeepEqual(rec.Sports, want) {
			t.Errorf("Sports = %v, want %v", rec.Sports, want)
		}
	})
	t.Run("two in a row both get the next record's time, in arrival order", func(t *testing.T) {
		rec := recordingOf(t, encodeFIT(t, fileID(), sportMesg(typedef.SportCycling), sportMesg(typedef.SportRunning), r(5)))
		want := []workout.RawSport{{At: at(5), Sport: "cycling"}, {At: at(5), Sport: "running"}}
		if !reflect.DeepEqual(rec.Sports, want) {
			t.Errorf("Sports = %v, want %v", rec.Sports, want)
		}
	})
	t.Run("one after the last record of a sequence is dropped and does not attach to the next sequence", func(t *testing.T) {
		a := encodeFIT(t, fileID(), sportMesg(typedef.SportRunning), r(0), r(1), sportMesg(typedef.SportCycling))
		b := encodeFIT(t, fileID(), r(100))
		rec := recordingOf(t, append(a, b...))
		if want := []workout.RawSport{{At: at(0), Sport: "running"}}; !reflect.DeepEqual(rec.Sports, want) {
			t.Errorf("Sports = %v, want %v", rec.Sports, want)
		}
	})
	t.Run("one in a skipped sequence is dropped", func(t *testing.T) {
		a := encodeFIT(t, fileID(), r(0))
		b := encodeFIT(t, settingsFileID(), sportMesg(typedef.SportSwimming), r(100))
		rec := recordingOf(t, append(a, b...))
		if len(rec.Sports) != 0 || rec.Integrity.SkippedSequences != 1 {
			t.Errorf("Sports = %v, integrity %+v", rec.Sports, rec.Integrity)
		}
	})
	t.Run("an invalid sport is empty", func(t *testing.T) {
		rec := recordingOf(t, encodeFIT(t, fileID(), mesgdef.NewSport(nil).SetSubSport(typedef.SubSportGeneric).ToMesg(nil), r(0)))
		if want := []workout.RawSport{{At: at(0), Sport: ""}}; !reflect.DeepEqual(rec.Sports, want) {
			t.Errorf("Sports = %v, want %v", rec.Sports, want)
		}
	})
	t.Run("lap.sport is read, invalid is empty", func(t *testing.T) {
		lap := func(sport typedef.Sport) proto.Message {
			l := mesgdef.NewLap(nil).SetTimestamp(at(9)).SetStartTime(at(0)).SetTotalElapsedTime(9000)
			if sport != typedef.SportInvalid {
				l.SetSport(sport)
			}
			return l.ToMesg(nil)
		}
		rec := recordingOf(t, encodeFIT(t, fileID(), r(0), lap(typedef.SportRunning), lap(typedef.SportInvalid)))
		if len(rec.Laps) != 2 || rec.Laps[0].Sport != "running" || rec.Laps[1].Sport != "" {
			t.Errorf("Laps = %+v", rec.Laps)
		}
	})
}

// TestSampleCountCorpus pins SampleCount to Normalise for the recording of
// every corpus file (QA-08-r1-6 a); rejected files give a recording whose
// count is whatever Normalise gives.
func TestSampleCountCorpus(t *testing.T) {
	n := 0
	for _, set := range corpustest.Sets {
		for _, path := range corpustest.Files(t, corpusDir, set) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !looksLikeFIT(b) {
				continue
			}
			rec := recordingOf(t, b)
			if got, want := workout.SampleCount(rec), len(workout.Normalise(rec).Samples.Offset); got != want {
				t.Errorf("%s/%s: SampleCount = %d, Normalise gives %d", set, filepath.Base(path), got, want)
			}
			n++
		}
	}
	if n < 30 {
		t.Errorf("only %d corpus files checked", n)
	}
}

// TestChainedSameSport (QA-08-r1-3, D-14, D-17): a ride saved in two cycling
// sequences is one cumulative distance stream with its real sport and a true
// average speed.
func TestChainedSameSport(t *testing.T) {
	w := mustParse(t, readSynthetic(t, "chained-ride-ride.fit"))
	if w.Sport != "cycling" || w.Integrity.Sequences != 2 || len(w.Sessions) != 2 {
		t.Fatalf("sport %q, %+v, %d sessions", w.Sport, w.Integrity, len(w.Sessions))
	}
	d := w.Samples.Distance
	for i := 1; i < len(d); i++ {
		if d[i] < d[i-1] {
			t.Fatalf("distance falls at sample %d: %v -> %v", i, d[i-1], d[i])
		}
	}
	rec := recordingOf(t, readSynthetic(t, "chained-ride-ride.fit"))
	var last [2]float64
	for i, r := range rec.Records {
		last[btoi(i >= rec.SequenceStarts[0])] = r.Distance
	}
	if w.Derived.Distance == nil || !within(*w.Derived.Distance, last[0]+last[1], 0.01) {
		t.Errorf("derived distance %v, want %v", deref(w.Derived.Distance), last[0]+last[1])
	}
	if w.Summary.AvgSpeed == nil || w.Derived.AvgSpeed == nil || *w.Summary.AvgSpeed != *w.Derived.AvgSpeed {
		t.Errorf("summary avg speed %v, derived %v", deref(w.Summary.AvgSpeed), deref(w.Derived.AvgSpeed))
	}
	if w.Summary.AvgSpeed != nil && (*w.Summary.AvgSpeed < 4.8 || *w.Summary.AvgSpeed > 5.2) {
		t.Errorf("average speed %v, want about 5 m/s", *w.Summary.AvgSpeed)
	}
	if w.Summary.MaxCadence == nil {
		t.Errorf("max cadence missing for a same-sport file")
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func within(a, b, eps float64) bool { return a-b <= eps && b-a <= eps }

// TestTruncatedRecordLeg: the cut tail of a multi-session file is part of
// the totals (QA-08-r1-2).
func TestTruncatedRecordLeg(t *testing.T) {
	full := mustParse(t, readSynthetic(t, "triathlon.fit"))
	cut := mustParse(t, readSynthetic(t, "triathlon-cut-90.fit"))
	if len(cut.Sessions) != 4 || !cut.Integrity.Truncated {
		t.Fatalf("%d sessions, %+v", len(cut.Sessions), cut.Integrity)
	}
	last := cut.Samples.Offset[len(cut.Samples.Offset)-1]
	if last <= cut.Sessions[3].End {
		t.Fatalf("the cut has no tail: last sample %d, T2 ends %d", last, cut.Sessions[3].End)
	}
	// The summary distance follows the data: at least the T2 sessions' sum,
	// and within a tail's rounding of the recorded distance.
	if cut.Summary.Distance == nil || cut.Derived.Distance == nil {
		t.Fatal("distance missing")
	}
	if diff := *cut.Derived.Distance - *cut.Summary.Distance; diff < -1 || diff > 50 {
		t.Errorf("summary distance %v, derived %v", *cut.Summary.Distance, *cut.Derived.Distance)
	}
	if *cut.Summary.Distance >= *full.Summary.Distance {
		t.Errorf("summary distance %v is not below the whole file's %v", *cut.Summary.Distance, *full.Summary.Distance)
	}
	if cut.Summary.AvgSpeed != nil || cut.Summary.AvgCadence != nil || cut.Summary.MaxCadence != nil {
		t.Errorf("mixed sports must hide average speed and cadence: %+v", cut.Summary)
	}
}

// TestRealTruncationLocal cuts every local real file (git-ignored, skipped
// when absent) and checks that each accepted cut keeps the file's sport and
// that every stream, cadence included, is a prefix of the whole file's
// stream (QA-08-r1-1 on real data). It prints nothing derived from the files.
func TestRealTruncationLocal(t *testing.T) {
	files := corpustest.Files(t, corpusDir, "real")
	if len(files) == 0 {
		t.Skip("no local real files")
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		full := mustParse(t, b)
		for pct := 5; pct <= 95; pct += 5 {
			w, err := testParser.Parse(context.Background(), b[:len(b)*pct/100])
			if err != nil {
				continue // too early to hold a record
			}
			n := len(w.Samples.Offset)
			if !w.Integrity.Truncated {
				t.Errorf("%s at %d %%: not truncated", filepath.Base(path), pct)
			}
			if full.Sport != "" && w.Sport != full.Sport && full.Sport != "multisport" {
				t.Errorf("%s at %d %%: sport %q, whole file %q", filepath.Base(path), pct, w.Sport, full.Sport)
			}
			got, want := streamsOf(w.Samples), streamsOf(full.Samples)
			for k := range got {
				if got[k] != nil && !sameNaN(got[k], want[k][:n]) {
					t.Errorf("%s at %d %%: stream %d is not a prefix of the whole file's", filepath.Base(path), pct, k)
				}
			}
		}
	}
}
