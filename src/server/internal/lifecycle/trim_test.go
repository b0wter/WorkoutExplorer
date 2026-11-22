package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"workout-explorer/internal/workout"
)

// F-47 TR-3, TR-4, TR-8 at the service.

type fakeTrimmer struct {
	calls    int
	tooShort bool
	view     OwnerView
	err      error
}

func (f *fakeTrimmer) ResolveTrim(_ context.Context, _ Record, s, e *time.Time) (*time.Time, *time.Time, error) {
	f.calls++
	if f.tooShort {
		return nil, nil, ErrTrimTooShort
	}
	if s != nil { // "snapped" one second inward
		x := s.Add(time.Second)
		s = &x
	}
	return s, e, nil
}

func (f *fakeTrimmer) CheckEdits(context.Context, Record, []Pin, []time.Time) ([]PinCheck, []LapCheck, error) {
	return nil, nil, errors.New("not used")
}

func (f *fakeTrimmer) TrimmedView(context.Context, Record) (OwnerView, error) { return f.view, f.err }

func TestUpdateTrim(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	ft := &fakeTrimmer{}
	s.d.Trimmer = ft
	ctx := context.Background()
	start := t0.Add(10 * time.Second)

	rec, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{TrimStart: OptTime{Set: true, At: &start}})
	if err != nil || rec.TrimStartAt == nil || !rec.TrimStartAt.Equal(start.Add(time.Second)) || rec.TrimEndAt != nil {
		t.Fatalf("%+v %v", rec, err)
	}
	// An unrelated change neither parses nor touches the trim.
	calls := ft.calls
	if rec, err = s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("x")}); err != nil || ft.calls != calls || rec.TrimStartAt == nil {
		t.Fatalf("%+v %v", rec, err)
	}
	// Too short: the plain sentence, nothing stored.
	ft.tooShort = true
	end := t0.Add(20 * time.Second)
	var ie *InvalidSettingsError
	if _, err = s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{TrimEnd: OptTime{Set: true, At: &end}}); !errors.As(err, &ie) || ie.Message != "At least 1 minute of the workout must remain." {
		t.Fatalf("%v", err)
	}
	for _, row := range r.rows {
		if row.TrimEndAt != nil {
			t.Fatal("a refused trim was stored")
		}
	}
	// Both null: the trim is removed without a parse.
	calls = ft.calls
	rec, err = s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{TrimStart: OptTime{Set: true}, TrimEnd: OptTime{Set: true}})
	if err != nil || rec.TrimStartAt != nil || ft.calls != calls {
		t.Fatalf("%+v %v", rec, err)
	}
}

func TestViewFailsSafe(t *testing.T) {
	s, _, _, _, _, _ := owned(t, nil)
	at := t0
	rec := Record{ID: "x", TrimEndAt: &at, HidingSteps: &workout.HidingSteps{Start: []int{0}, End: []int{1}}}
	s.d.Trimmer = &fakeTrimmer{err: errors.New("boom")}
	if v := s.View(context.Background(), rec); v.Trimmed != nil || v.Steps != nil || v.Suggested != "" {
		t.Errorf("%+v", v)
	}
	s.d.Trimmer = &fakeTrimmer{view: OwnerView{Suggested: "s", Steps: &workout.HidingSteps{Start: []int{3}, End: []int{4}}, Trimmed: &TrimmedView{Sport: "running"}}}
	if v := s.View(context.Background(), rec); v.Trimmed == nil || v.Suggested != "s" || v.Steps.Start[0] != 3 {
		t.Errorf("%+v", v)
	}
	if v := s.View(context.Background(), Record{ID: "y", HidingSteps: &workout.HidingSteps{Start: []int{0}, End: []int{1}}}); v.Trimmed != nil || v.Steps == nil || v.Steps.End[0] != 1 {
		t.Errorf("untrimmed: %+v", v)
	}
}
