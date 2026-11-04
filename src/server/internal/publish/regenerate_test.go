package publish

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

// rows is a shared row table, standing in for the database both processes see.
type rows struct {
	mu sync.Mutex
	m  map[string]lifecycle.Record
}

func (r *rows) WorkoutByPublicID(_ context.Context, pub string) (lifecycle.Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.m[pub]
	return rec, ok, nil
}

func (r *rows) set(rec lifecycle.Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[rec.PublicID] = rec
}

type hookOriginals struct {
	b      []byte
	before func() // called once, at the first read
	once   sync.Once
}

func (o *hookOriginals) Original(string) ([]byte, error) {
	if o.before != nil {
		o.once.Do(o.before)
	}
	return o.b, nil
}

// live is a published record that is live now (the re-read uses the clock).
func live() lifecycle.Record {
	r := draft()
	r.CreatedAt = time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	r.ExpiresAt = r.CreatedAt.Add(7 * 24 * time.Hour)
	return published(r)
}

// TestRecheckUnderLock (F-30 RG-5, the F-21 hand-off probe): an owner's
// hides-more save in serve lands while regenerate builds; regenerate's
// write phase then finds a different row and writes nothing, so the page is
// the more hidden one. Two Areas on one directory are the two processes.
func TestRecheckUnderLock(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	dir := filepath.Dir(fx.root)
	rs := &rows{m: map[string]lifecycle.Record{}}
	p := live()
	rs.set(p)
	serve, err := Open(dir, Deps{Template: fx.a.d.Template, Originals: originals{id: fx.orig}, Parser: fx.a.d.Parser, BaseURL: testBaseURL, Rows: rs})
	if err != nil {
		t.Fatal(err)
	}
	if err := serve.WorkoutChanged(context.Background(), draft(), p); err != nil {
		t.Fatal(err)
	}
	hidden := p
	hidden.HideEnds = true
	orig := &hookOriginals{b: fx.orig}
	orig.before = func() {
		rs.set(hidden) // serve commits the row, then builds
		if err := serve.WorkoutChanged(context.Background(), p, hidden); err != nil {
			t.Error(err)
		}
	}
	regen, err := Open(dir, Deps{Template: fx.a.d.Template, Originals: orig, Parser: fx.a.d.Parser, BaseURL: testBaseURL, Rows: rs})
	if err != nil {
		t.Fatal(err)
	}
	after := p
	after.SummaryJSON = `{"regenerated":true}` // regenerate's (b), committed before serve's save
	if err := regen.WorkoutChanged(context.Background(), p, after); err != nil {
		t.Fatal(err)
	}
	served, err := os.ReadFile(filepath.Join(fx.folder(), indexName))
	if err != nil {
		t.Fatal(err)
	}
	// The page is exactly what serve builds for the hidden row.
	b, err := serve.build(context.Background(), hidden)
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != string(b.index) {
		t.Error("the stale build overwrote the more hidden page")
	}

	// Gone, or no longer live: nothing is written.
	for _, c := range []struct {
		name string
		edit func()
	}{
		{"gone", func() { rs.mu.Lock(); delete(rs.m, p.PublicID); rs.mu.Unlock() }},
		{"due", func() { d := hidden; d.ExpiresAt = time.Now().Add(-time.Second); rs.set(d) }},
	} {
		c.edit()
		os.RemoveAll(fx.folder())
		if err := regen.WorkoutChanged(context.Background(), hidden, hidden); err != nil {
			t.Fatal(err)
		}
		if names := fx.names(t); names != nil {
			t.Errorf("%s: wrote %v", c.name, names)
		}
	}
}

// TestRegenerateInPlace (RG-6): a run changes only derived fields, so the
// page is rebuilt in place: index.html is never removed.
func TestRegenerateInPlace(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	rs := &rows{m: map[string]lifecycle.Record{}}
	fx.a.d.Rows = rs
	p := live()
	rs.set(p)
	fx.change(t, draft(), p)
	after := p
	after.SummaryJSON, after.Sport, after.HiddenRanges = `{"x":1}`, "running", [][2]int{{0, 3}}
	rs.set(after)
	fx.steps = nil
	fx.change(t, p, after)
	if slices.Contains(fx.steps, "remove "+indexName) {
		t.Errorf("steps %v", fx.steps)
	}
	if name, _ := fx.complete(t); name == "" {
		t.Error("no page")
	}
}

// TestSameRecord: every field counts; times compare by instant.
func TestSameRecord(t *testing.T) {
	p := live()
	q := p
	at := p.PublishedAt.In(time.FixedZone("x", 3600))
	q.PublishedAt = &at
	q.CreatedAt = q.CreatedAt.Local()
	if !sameRecord(p, q) {
		t.Error("same instants differ")
	}
	v := reflect.ValueOf(&q).Elem()
	for i := 0; i < v.NumField(); i++ {
		r := p
		f := reflect.ValueOf(&r).Elem().Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(f.String() + "x")
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.Array:
			f.Index(0).SetUint(f.Index(0).Uint() + 1)
		case reflect.Pointer:
			f.Set(reflect.Zero(f.Type()))
			if reflect.DeepEqual(r, p) { // was nil already
				switch f.Type().Elem().Kind() {
				case reflect.Int:
					n := 1
					f.Set(reflect.ValueOf(&n))
				default:
					tm := time.Now()
					f.Set(reflect.ValueOf(&tm))
				}
			}
		case reflect.Slice:
			f.Set(reflect.ValueOf([][2]int{{1, 2}}))
		case reflect.Struct:
			f.Set(reflect.ValueOf(time.Now()))
		default:
			t.Fatalf("field %s: kind %s not covered", v.Type().Field(i).Name, f.Kind())
		}
		if sameRecord(p, r) {
			t.Errorf("field %s: a change is not seen", v.Type().Field(i).Name)
		}
	}
}

// TestAssets (RG-9): every /assets/ name any page links, once.
func TestAssets(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	for pub, page := range map[string]string{
		"AAAAAAAAAAAAAAAAAAAAAA": `<script src="/assets/main-1.js"></script><link href="/assets/main-1.css">`,
		"BBBBBBBBBBBBBBBBBBBBBB": `<script src="/assets/main-2.js"></script><link href="/assets/main-1.css"> /assets/ /assets/.tmp`,
	} {
		if err := os.MkdirAll(filepath.Join(fx.root, pub), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fx.root, pub, indexName), []byte(page), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(fx.root, "CCCCCCCCCCCCCCCCCCCCCC"), 0o750) // no page yet (remove-first)
	got, err := fx.a.Assets()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"main-1.css", "main-1.js", "main-2.js"}; !reflect.DeepEqual(got, want) {
		t.Errorf("%v", got)
	}
}
