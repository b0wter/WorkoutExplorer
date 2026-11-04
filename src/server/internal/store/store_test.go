package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

var ctx = context.Background()

func openTemp(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workouts.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func scalar[T any](t *testing.T, db *DB, q string) T {
	t.Helper()
	var v T
	if err := db.db.QueryRowContext(ctx, q).Scan(&v); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return v
}

func TestOpen(t *testing.T) {
	db, path := openTemp(t)
	if v := scalar[int](t, db, "PRAGMA user_version"); v != 4 {
		t.Errorf("user_version = %d, want 4", v)
	}
	if m := scalar[string](t, db, "PRAGMA journal_mode"); m != "wal" {
		t.Errorf("journal_mode = %q", m)
	}
	if v := scalar[int](t, db, "PRAGMA foreign_keys"); v != 1 {
		t.Errorf("foreign_keys = %d", v)
	}
	if v := scalar[int](t, db, "PRAGMA secure_delete"); v != 1 {
		t.Errorf("secure_delete = %d", v)
	}
	if v := scalar[int](t, db, "PRAGMA busy_timeout"); v != 5000 {
		t.Errorf("busy_timeout = %d", v)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if st, err := os.Stat(p); err == nil && st.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", p, st.Mode().Perm())
		}
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("database file: %v, %v", st, err)
	}
	// Opening again applies nothing.
	if _, err := db.db.ExecContext(ctx, "CREATE TABLE marker (x)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if n := scalar[int](t, db2, "SELECT count(*) FROM sqlite_master WHERE name = 'marker'"); n != 1 {
		t.Errorf("reopening dropped data")
	}
	if v := scalar[int](t, db2, "PRAGMA user_version"); v != 4 {
		t.Errorf("user_version = %d after reopening", v)
	}
}

// TestOpenPathNeedsEscaping: a path with characters that matter in a URI.
func TestOpenPathNeedsEscaping(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a b#c%d?e")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, filepath.Join(dir, "workouts.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := os.Stat(filepath.Join(dir, "workouts.db")); err != nil {
		t.Errorf("the database is not at the given path: %v", err)
	}
}

// TestSchema (AC 5): the exact columns (16 of v1, 4 of v2: ADR-030) and no other table.
func TestSchema(t *testing.T) {
	db, _ := openTemp(t)
	want := []struct {
		name, typ string
		notNull   bool
	}{
		{"id", "TEXT", true}, {"public_id", "TEXT", true}, {"edit_token_hash", "BLOB", true},
		{"title", "TEXT", true}, {"description", "TEXT", true}, {"sport", "TEXT", true},
		{"start_time", "INTEGER", false}, {"time_zone_offset_s", "INTEGER", false},
		{"visibility", "TEXT", true}, {"hide_ends", "INTEGER", true}, {"allow_download", "INTEGER", true},
		{"summary_json", "TEXT", true}, {"data_end_s", "INTEGER", false}, {"created_at", "INTEGER", true},
		{"published_at", "INTEGER", false}, {"expires_at", "INTEGER", true},
		{"hidden1_first", "INTEGER", false}, {"hidden1_last", "INTEGER", false},
		{"hidden2_first", "INTEGER", false}, {"hidden2_last", "INTEGER", false},
	}
	rows, err := db.db.QueryContext(ctx, "PRAGMA table_info(workouts)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) {
			t.Fatalf("extra column %s", name)
		}
		if w := want[i]; name != w.name || typ != w.typ || (notNull == 1) != w.notNull {
			t.Errorf("column %d = %s %s notnull=%d, want %+v", i, name, typ, notNull, w)
		}
		i++
	}
	if i != len(want) {
		t.Errorf("%d columns, want %d", i, len(want))
	}
	trows, err := db.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatal(err)
	}
	defer trows.Close()
	var tables []string
	for trows.Next() {
		var n string
		trows.Scan(&n)
		tables = append(tables, n)
	}
	if !reflect.DeepEqual(tables, []string{"workouts", "reports"}) {
		t.Errorf("tables = %v", tables)
	}
	// F-27 R-2, R-3: reason, message and time only; nothing about the request.
	if got := columnsOf(t, db, "reports"); !reflect.DeepEqual(got, []string{
		"id INTEGER pk", "workout_id TEXT notnull", "reason TEXT notnull", "message TEXT notnull", "created_at INTEGER notnull",
	}) {
		t.Errorf("reports columns %v", got)
	}
	var fk string
	if err := db.db.QueryRowContext(ctx, `SELECT "table" || '.' || "to" || ' ' || on_delete FROM pragma_foreign_key_list('reports')`).Scan(&fk); err != nil || fk != "workouts.id CASCADE" {
		t.Errorf("reports foreign key %q, %v", fk, err)
	}
}

// columnsOf lists a table's columns as "name TYPE[ notnull][ pk]".
func columnsOf(t *testing.T, db *DB, table string) []string {
	t.Helper()
	rows, err := db.db.QueryContext(ctx, "SELECT name, type, \"notnull\", pk FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name, typ string
		var notNull, pk int
		if err := rows.Scan(&name, &typ, &notNull, &pk); err != nil {
			t.Fatal(err)
		}
		c := name + " " + typ
		if notNull == 1 {
			c += " notnull"
		}
		if pk > 0 {
			c += " pk"
		}
		out = append(out, c)
	}
	return out
}

func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workouts.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Open(ctx, path)
	if err == nil || !strings.Contains(err.Error(), "newer than this binary") {
		t.Fatalf("Open err = %v", err)
	}
	// Nothing was applied: still version 99 and no table.
	raw, err = sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var v, n int
	raw.QueryRow("PRAGMA user_version").Scan(&v)
	raw.QueryRow("SELECT count(*) FROM sqlite_master").Scan(&n)
	if v != 99 || n != 0 {
		t.Errorf("user_version %d, %d schema objects after a refused Open", v, n)
	}
}

func TestMigrationFiles(t *testing.T) {
	ms, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 1 {
		t.Fatal("no migrations")
	}
	for i, m := range ms {
		if m.version != i+1 || !strings.HasPrefix(m.name, fmt.Sprintf("%04d_", i+1)) {
			t.Errorf("migration %d is %s", i, m.name)
		}
	}
}

func record(n int) lifecycle.Record {
	created := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return lifecycle.Record{
		ID:            fmt.Sprintf("%032x", n),
		PublicID:      fmt.Sprintf("%022d", n),
		EditTokenHash: [32]byte{1, 2, 3},
		Sport:         "cycling",
		Visibility:    lifecycle.Draft,
		SummaryJSON:   `{"distance":1}`,
		CreatedAt:     created,
		ExpiresAt:     created.Add(lifecycle.DefaultLifetime),
	}
}

func TestInsertWorkout(t *testing.T) {
	db, _ := openTemp(t)
	start := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	off, end := -3600, 321
	full := record(1)
	full.Title, full.Description = "Title", "Desc"
	full.StartTime, full.TimeZoneOffsetS, full.DataEndS = &start, &off, &end
	if err := db.InsertWorkout(ctx, full); err != nil {
		t.Fatal(err)
	}
	var (
		id, pub, title, desc, sport, vis, sum string
		hash                                  []byte
		st, tz, de, pa                        sql.NullInt64
		he, ad, ca, ea                        int64
	)
	err := db.db.QueryRowContext(ctx, `SELECT id, public_id, edit_token_hash, title, description, sport, start_time, time_zone_offset_s,
		visibility, hide_ends, allow_download, summary_json, data_end_s, created_at, published_at, expires_at FROM workouts`).
		Scan(&id, &pub, &hash, &title, &desc, &sport, &st, &tz, &vis, &he, &ad, &sum, &de, &ca, &pa, &ea)
	if err != nil {
		t.Fatal(err)
	}
	if id != full.ID || pub != full.PublicID || !bytes.Equal(hash, full.EditTokenHash[:]) || title != "Title" || desc != "Desc" ||
		sport != "cycling" || st.Int64 != start.Unix() || tz.Int64 != -3600 || vis != "draft" || he != 0 || ad != 0 ||
		sum != `{"distance":1}` || de.Int64 != 321 || ca != full.CreatedAt.Unix() || pa.Valid || ea != ca+604800 {
		t.Errorf("row = %v %v %x %v %v %v %v %v %v %v %v %v %v %v %v %v", id, pub, hash, title, desc, sport, st, tz, vis, he, ad, sum, de, ca, pa, ea)
	}
	// NULLs.
	if err := db.InsertWorkout(ctx, record(2)); err != nil {
		t.Fatal(err)
	}
	if err := db.db.QueryRowContext(ctx, "SELECT start_time, time_zone_offset_s, data_end_s FROM workouts WHERE id = ?", record(2).ID).Scan(&st, &tz, &de); err != nil {
		t.Fatal(err)
	}
	if st.Valid || tz.Valid || de.Valid {
		t.Errorf("NULLs became values: %v %v %v", st, tz, de)
	}
	// Duplicate public_id.
	dup := record(3)
	dup.PublicID = full.PublicID
	if err := db.InsertWorkout(ctx, dup); err == nil {
		t.Errorf("duplicate public_id accepted")
	}

	pub0 := time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC)
	mut := map[string]func(*lifecycle.Record){
		"bad visibility":                func(r *lifecycle.Record) { r.Visibility = "public" },
		"private (ADR-019 item 8)":      func(r *lifecycle.Record) { r.Visibility = "private"; r.PublishedAt = &pub0 },
		"draft with published_at":       func(r *lifecycle.Record) { r.PublishedAt = &pub0 },
		"unlisted without published_at": func(r *lifecycle.Record) { r.Visibility = lifecycle.Unlisted },
		"30 days + 1 s":                 func(r *lifecycle.Record) { r.ExpiresAt = r.CreatedAt.Add(lifecycle.MaxLifetime + time.Second) },
		"expires at created":            func(r *lifecycle.Record) { r.ExpiresAt = r.CreatedAt },
		"short hash":                    nil, // hash is a fixed array: checked with raw SQL below
		"invalid JSON":                  func(r *lifecycle.Record) { r.SummaryJSON = "{nope" },
		"negative data_end":             func(r *lifecycle.Record) { n := -1; r.DataEndS = &n },
	}
	i := 10
	for name, f := range mut {
		if f == nil {
			continue
		}
		i++
		r := record(i)
		f(&r)
		if err := db.InsertWorkout(ctx, r); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// 30 days exactly is fine.
	r := record(50)
	r.ExpiresAt = r.CreatedAt.Add(lifecycle.MaxLifetime)
	if err := db.InsertWorkout(ctx, r); err != nil {
		t.Errorf("30 days: %v", err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO workouts (id, public_id, edit_token_hash, sport, visibility, summary_json, created_at, expires_at)
		VALUES (?, ?, x'0102', 'x', 'draft', '{}', 1, 2)`, strings.Repeat("e", 32), strings.Repeat("e", 22)); err == nil {
		t.Errorf("short hash accepted")
	}
}

func TestWorkoutIDs(t *testing.T) {
	db, _ := openTemp(t)
	got, err := db.WorkoutIDs(ctx)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: %v, %v", got, err)
	}
	for i := 1; i <= 3; i++ {
		if err := db.InsertWorkout(ctx, record(i)); err != nil {
			t.Fatal(err)
		}
	}
	got, err = db.WorkoutIDs(ctx)
	if err != nil || len(got) != 3 {
		t.Fatalf("ids = %v, %v", got, err)
	}
	for i := 1; i <= 3; i++ {
		if _, ok := got[record(i).ID]; !ok {
			t.Errorf("id %d missing", i)
		}
	}
}

func TestWorkoutExists(t *testing.T) {
	db, _ := openTemp(t)
	if err := db.InsertWorkout(ctx, record(1)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want bool
	}{{record(1).ID, true}, {record(2).ID, false}, {"", false}} {
		got, err := db.WorkoutExists(ctx, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("WorkoutExists(%q) = %v, %v; want %v", tc.id, got, err, tc.want)
		}
	}
	// A failing query is an error, not "absent".
	db.Close()
	if _, err := db.WorkoutExists(ctx, record(1).ID); err == nil {
		t.Errorf("a closed database reported no error")
	}
}

func TestDeleteWorkout(t *testing.T) {
	db, _ := openTemp(t)
	for i := 1; i <= 3; i++ {
		if err := db.InsertWorkout(ctx, record(i)); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := db.DeleteWorkout(ctx, record(2).ID); err != nil || !ok {
		t.Fatal(ok, err)
	}
	got, err := db.WorkoutIDs(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("ids = %v, %v", got, err)
	}
	if _, gone := got[record(2).ID]; gone {
		t.Errorf("the deleted row is still there")
	}
	for _, i := range []int{1, 3} {
		if _, ok := got[record(i).ID]; !ok {
			t.Errorf("row %d was touched", i)
		}
	}
	// An absent ID is not an error.
	if ok, err := db.DeleteWorkout(ctx, record(2).ID); err != nil || ok {
		t.Errorf("deleting an absent row: %v, %v", ok, err)
	}
	if ok, err := db.DeleteWorkout(ctx, "no-such-id"); err != nil || ok {
		t.Errorf("deleting an unknown ID: %v, %v", ok, err)
	}
}

// TestCheckpoint (QA-21-r1-1): after a delete and a checkpoint neither the
// database file nor the WAL holds the row; a reader that keeps the checkpoint
// from completing gives an error.
func TestCheckpoint(t *testing.T) {
	db, path := openTemp(t)
	r := record(1)
	r.Title = "CHECKPOINTMARKER"
	if err := db.InsertWorkout(ctx, r); err != nil {
		t.Fatal(err)
	}
	reader, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow("SELECT count(*) FROM workouts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DeleteWorkout(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, "PRAGMA busy_timeout = 50"); err != nil {
		t.Fatal(err)
	}
	if err := db.Checkpoint(ctx); err == nil {
		t.Error("checkpoint completed under an open reader")
	}
	tx.Rollback()
	if err := db.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + "-wal"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(r.Title)) || bytes.Contains(b, []byte(r.PublicID)) {
			t.Errorf("%s still holds the row", filepath.Base(p))
		}
	}
}

func TestWorkoutByPublicID(t *testing.T) {
	db, _ := openTemp(t)
	r := record(1)
	st := time.Unix(1_700_000_000, 0).UTC()
	off, end := 3600, 42
	r.StartTime, r.TimeZoneOffsetS, r.DataEndS = &st, &off, &end
	if err := db.InsertWorkout(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.WorkoutByPublicID(ctx, r.PublicID)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if got.ID != r.ID || got.EditTokenHash != r.EditTokenHash || !got.StartTime.Equal(st) ||
		*got.TimeZoneOffsetS != 3600 || *got.DataEndS != 42 || got.Visibility != r.Visibility ||
		!got.CreatedAt.Equal(r.CreatedAt) || !got.ExpiresAt.Equal(r.ExpiresAt) || got.PublishedAt != nil {
		t.Errorf("got %+v", got)
	}
	if _, ok, err := db.WorkoutByPublicID(ctx, "nope"); ok || err != nil {
		t.Errorf("absent: %v %v", ok, err)
	}
}

func TestUpdateWorkout(t *testing.T) {
	db, _ := openTemp(t)
	r := record(2)
	if err := db.InsertWorkout(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, found, err := db.UpdateWorkout(ctx, r.ID, func(c lifecycle.Record) (lifecycle.Record, error) {
		c.Title, c.HideEnds = "T", true
		c.ExpiresAt = c.CreatedAt.Add(30 * 24 * time.Hour)
		return c, nil
	})
	if err != nil || !found || got.Title != "T" {
		t.Fatal(got, found, err)
	}
	back, _, _ := db.WorkoutByPublicID(ctx, r.PublicID)
	if back.Title != "T" || !back.HideEnds || !back.ExpiresAt.Equal(r.CreatedAt.Add(30*24*time.Hour)) || back.Sport != r.Sport {
		t.Errorf("stored %+v", back)
	}
	// An error from apply stores nothing and is returned unchanged.
	sentinel := fmt.Errorf("refused")
	_, _, err = db.UpdateWorkout(ctx, r.ID, func(c lifecycle.Record) (lifecycle.Record, error) {
		c.Title = "X"
		return c, sentinel
	})
	if err != sentinel {
		t.Errorf("err %v", err)
	}
	if back, _, _ = db.WorkoutByPublicID(ctx, r.PublicID); back.Title != "T" {
		t.Error("stored despite error")
	}
	// A CHECK violation (30 days + 1 s) is an error and stores nothing.
	_, _, err = db.UpdateWorkout(ctx, r.ID, func(c lifecycle.Record) (lifecycle.Record, error) {
		c.ExpiresAt = c.CreatedAt.Add(30*24*time.Hour + time.Second)
		return c, nil
	})
	if err == nil {
		t.Error("check violation accepted")
	}
	// A missing row: not found, apply not called.
	called := false
	_, found, err = db.UpdateWorkout(ctx, "missing", func(c lifecycle.Record) (lifecycle.Record, error) { called = true; return c, nil })
	if found || err != nil || called {
		t.Errorf("missing: %v %v %v", found, err, called)
	}
}

// TestUpdateDerived (F-30 RG-4 (b)): only the derived columns change, and
// the row afterwards is returned; a missing row is not found.
func TestUpdateDerived(t *testing.T) {
	db, _ := openTemp(t)
	r := published(3)
	r.Title, r.HideEnds, r.AllowDownload = "kept", true, true
	if err := db.InsertWorkout(ctx, r); err != nil {
		t.Fatal(err)
	}
	d := lifecycle.Record{Title: "ignored", Sport: "running", SummaryJSON: `{"n":2}`, HiddenRanges: [][2]int{{1, 2}, {8, 9}}}
	st, off, end := r.CreatedAt.Add(-time.Hour), 3600, 42
	d.StartTime, d.TimeZoneOffsetS, d.DataEndS = &st, &off, &end
	got, found, err := db.UpdateDerived(ctx, r.ID, d, r.CreatedAt)
	if err != nil || !found {
		t.Fatal(found, err)
	}
	want := r
	want.Sport, want.SummaryJSON, want.HiddenRanges = d.Sport, d.SummaryJSON, d.HiddenRanges
	want.StartTime, want.TimeZoneOffsetS, want.DataEndS = &st, &off, &end
	back, _, _ := db.WorkoutByPublicID(ctx, r.PublicID)
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(back, want) {
		t.Errorf("got\n%+v\nstored\n%+v\nwant\n%+v", got, back, want)
	}
	// Back to no start time and no ranges: NULLs.
	if got, _, err = db.UpdateDerived(ctx, r.ID, lifecycle.Record{Sport: "x", SummaryJSON: "{}"}, r.CreatedAt); err != nil || got.StartTime != nil || got.HiddenRanges != nil || got.DataEndS != nil {
		t.Errorf("%+v %v", got, err)
	}
	if _, found, err := db.UpdateDerived(ctx, "missing", d, r.CreatedAt); found || err != nil {
		t.Errorf("missing: %v %v", found, err)
	}
}

// TestHiddenRanges (ADR-030): the ranges round-trip, are kept by
// UpdateWorkout, and malformed column values are refused.
func TestHiddenRanges(t *testing.T) {
	db, _ := openTemp(t)
	for i, want := range [][][2]int{nil, {{0, 99}}, {{0, 9}, {40, 99}}} {
		r := record(100 + i)
		r.HiddenRanges = want
		if err := db.InsertWorkout(ctx, r); err != nil {
			t.Fatal(want, err)
		}
		got, _, err := db.WorkoutByPublicID(ctx, r.PublicID)
		if err != nil || !reflect.DeepEqual(got.HiddenRanges, want) {
			t.Errorf("%v: got %v, %v", want, got.HiddenRanges, err)
		}
		up, _, err := db.UpdateWorkout(ctx, r.ID, func(c lifecycle.Record) (lifecycle.Record, error) {
			c.Title, c.HiddenRanges = "T", [][2]int{{1, 2}} // UpdateWorkout never writes the ranges
			return c, nil
		})
		if err != nil || up.Title != "T" {
			t.Fatal(err)
		}
		if got, _, _ = db.WorkoutByPublicID(ctx, r.PublicID); !reflect.DeepEqual(got.HiddenRanges, want) {
			t.Errorf("%v after update: %v", want, got.HiddenRanges)
		}
	}
	for i, bad := range [][4]any{
		{-1, 5, nil, nil}, {5, nil, nil, nil}, {nil, 5, nil, nil}, {5, 4, nil, nil},
		{nil, nil, 3, 4}, {0, 5, 6, 9}, {0, 5, 3, 9}, {0, 5, 8, nil}, {0, 5, 8, 7},
	} {
		r := record(200 + i)
		_, err := db.db.ExecContext(ctx, `INSERT INTO workouts (id, public_id, edit_token_hash, sport, visibility, summary_json,
			created_at, expires_at, hidden1_first, hidden1_last, hidden2_first, hidden2_last)
			VALUES (?, ?, ?, 'x', 'draft', '{}', 1, 2, ?, ?, ?, ?)`, r.ID, r.PublicID, r.EditTokenHash[:], bad[0], bad[1], bad[2], bad[3])
		if err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

// TestMigrateV1 (ADR-019 item 5, ADR-030 item 2): a v1 database with a row
// is migrated to v2, and the row reads as "no ranges".
func TestMigrateV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workouts.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	r := record(1)
	for _, q := range []string{ms[0].sql, "PRAGMA user_version = 1", "PRAGMA journal_mode = WAL"} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec(`INSERT INTO workouts (id, public_id, edit_token_hash, sport, visibility, summary_json, created_at, expires_at)
		VALUES (?, ?, ?, 'x', 'draft', '{}', 1, 2)`, r.ID, r.PublicID, r.EditTokenHash[:]); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v := scalar[int](t, db, "PRAGMA user_version"); v != 4 {
		t.Errorf("user_version = %d", v)
	}
	got, ok, err := db.WorkoutByPublicID(ctx, r.PublicID)
	if err != nil || !ok || got.HiddenRanges != nil {
		t.Errorf("%v %v %v", got.HiddenRanges, ok, err)
	}
}

// TestMigrateNoPrivate (ADR-019 item 8): a v2 database with draft, unlisted
// and private rows is migrated to v3. Private rows are dropped, never turned
// unlisted; the others are copied unchanged; the CHECK refuses private.
func TestMigrateNoPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workouts.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{ms[0].sql, ms[1].sql, "PRAGMA user_version = 2", "PRAGMA journal_mode = WAL"} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ins := `INSERT INTO workouts (id, public_id, edit_token_hash, title, description, sport, start_time, time_zone_offset_s,
		visibility, hide_ends, allow_download, summary_json, data_end_s, created_at, published_at, expires_at,
		hidden1_first, hidden1_last, hidden2_first, hidden2_last) VALUES (?, ?, ?, ?, 'd', 'cycling', 5, 3600, ?, 1, 1, '{"a":1}', 7, 10, ?, 20, ?, ?, ?, ?)`
	rows := []struct {
		vis       string
		published any
		hidden    []any
	}{
		{"draft", nil, []any{nil, nil, nil, nil}},
		{"unlisted", 11, []any{0, 4, 9, 12}},
		{"private", 11, []any{0, 4, nil, nil}},
		{"unlisted", 12, []any{nil, nil, nil, nil}},
	}
	for i, r := range rows {
		rec := record(i + 1)
		args := append([]any{rec.ID, rec.PublicID, rec.EditTokenHash[:], r.vis, r.vis, r.published}, r.hidden...)
		if _, err := raw.Exec(ins, args...); err != nil {
			t.Fatal(err)
		}
	}
	dump := func(db *sql.DB, where string) []string {
		rs, err := db.Query("SELECT quote(id) || '|' || quote(public_id) || '|' || quote(edit_token_hash) || '|' || quote(title) || '|' || quote(description) || '|' || quote(sport) || '|' || quote(start_time) || '|' || quote(time_zone_offset_s) || '|' || quote(visibility) || '|' || quote(hide_ends) || '|' || quote(allow_download) || '|' || quote(summary_json) || '|' || quote(data_end_s) || '|' || quote(created_at) || '|' || quote(published_at) || '|' || quote(expires_at) || '|' || quote(hidden1_first) || '|' || quote(hidden1_last) || '|' || quote(hidden2_first) || '|' || quote(hidden2_last) FROM workouts " + where + " ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		defer rs.Close()
		var out []string
		for rs.Next() {
			var s string
			rs.Scan(&s)
			out = append(out, s)
		}
		return out
	}
	want := dump(raw, "WHERE visibility <> 'private'")
	raw.Close()

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v := scalar[int](t, db, "PRAGMA user_version"); v != 4 {
		t.Errorf("user_version = %d", v)
	}
	if got := dump(db.db, ""); len(want) != 3 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows after migration:\n%v\nwant\n%v", got, want)
	}
	if _, ok, err := db.WorkoutByPublicID(ctx, record(3).PublicID); ok || err != nil {
		t.Errorf("private row kept: %v %v", ok, err)
	}
	if n := scalar[int](t, db, "SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'workouts_expires_at'"); n != 1 {
		t.Error("expiry index missing")
	}
	r := record(9)
	if _, err := db.db.ExecContext(ctx, ins, append([]any{r.ID, r.PublicID, r.EditTokenHash[:], "x", "private", 11}, nil, nil, nil, nil)...); err == nil {
		t.Error("CHECK accepts private")
	}
}

// TestLivePublicIDs (F-15 P-8): unlisted rows that have not expired, nothing else.
func TestLivePublicIDs(t *testing.T) {
	db, _ := openTemp(t)
	published := func(n int, v lifecycle.Visibility, expires time.Duration) lifecycle.Record {
		r := record(n)
		r.Visibility = v
		if v != lifecycle.Draft {
			p := r.CreatedAt
			r.PublishedAt = &p
		}
		r.ExpiresAt = r.CreatedAt.Add(expires)
		return r
	}
	rows := []lifecycle.Record{
		published(1, lifecycle.Unlisted, 7*24*time.Hour),
		published(3, lifecycle.Draft, 7*24*time.Hour),
		published(4, lifecycle.Unlisted, time.Hour), // expired at now
	}
	for _, r := range rows {
		if err := db.InsertWorkout(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	now := rows[0].CreatedAt.Add(time.Hour)
	got, err := db.LivePublicIDs(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if _, ok := got[rows[0].PublicID]; !ok {
		t.Errorf("got %v", got)
	}
}

// TestDeadlineBoundary (F-21 X-2) pins the SQL form of the deadline rule to
// lifecycle.Record.Live: at deadline - 1 s and at the deadline, a row is in
// DueWorkouts exactly when it is not Live, and in LivePublicIDs exactly when
// it is unlisted and Live.
func TestDeadlineBoundary(t *testing.T) {
	type tc struct {
		name     string
		vis      lifecycle.Visibility
		publish  time.Duration // after created_at
		lifetime time.Duration
		deadline time.Duration
	}
	day := 24 * time.Hour
	cases := []tc{
		{"unlisted 7 days", lifecycle.Unlisted, 0, 7 * day, 7 * day},
		{"draft 7 days", lifecycle.Draft, 0, 7 * day, 48 * time.Hour},
		{"draft 1 day", lifecycle.Draft, 0, day, day},
		{"draft 2 days", lifecycle.Draft, 0, 2 * day, 48 * time.Hour},
		{"draft 30 days", lifecycle.Draft, 0, 30 * day, 48 * time.Hour},
		{"published at 47 h, 7 days", lifecycle.Unlisted, 47 * time.Hour, 7 * day, 7 * day},
		{"unlisted 1 day", lifecycle.Unlisted, 0, day, day},
	}
	for i, c := range cases {
		db, _ := openTemp(t)
		r := record(i + 1)
		r.Visibility, r.ExpiresAt = c.vis, r.CreatedAt.Add(c.lifetime)
		if c.vis == lifecycle.Unlisted {
			p := r.CreatedAt.Add(c.publish)
			r.PublishedAt = &p
		}
		if err := db.InsertWorkout(ctx, r); err != nil {
			t.Fatal(err)
		}
		d := r.CreatedAt.Add(c.deadline)
		if !r.Deadline().Equal(d) {
			t.Fatalf("%s: pure deadline %v, want %v", c.name, r.Deadline(), d)
		}
		for _, now := range []time.Time{d.Add(-time.Second), d.Add(-time.Millisecond), d, d.Add(time.Millisecond)} {
			due, err := db.DueWorkouts(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if inDue := len(due) == 1 && due[0].ID == r.ID; inDue == r.Live(now) || len(due) > 1 {
				t.Errorf("%s at deadline %+v: due %v, live %v", c.name, now.Sub(d), inDue, r.Live(now))
			}
			if len(due) == 1 && !reflect.DeepEqual(due[0], r) {
				t.Errorf("%s: due row %+v, want %+v", c.name, due[0], r)
			}
			live, err := db.LivePublicIDs(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := live[r.PublicID]; ok != (r.Visibility == lifecycle.Unlisted && r.Live(now)) {
				t.Errorf("%s at deadline %+v: live public %v", c.name, now.Sub(d), ok)
			}
			// F-30 RG-2, RG-4 (b): the run's list and its row update use the same rule.
			all, err := db.LiveWorkouts(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if inLive := len(all) == 1 && all[0].ID == r.ID; inLive != r.Live(now) {
				t.Errorf("%s at deadline %+v: live workouts %v", c.name, now.Sub(d), inLive)
			}
			if _, found, err := db.UpdateDerived(ctx, r.ID, r, now); err != nil || found != r.Live(now) {
				t.Errorf("%s at deadline %+v: update derived %v %v", c.name, now.Sub(d), found, err)
			}
			// F-27 R-5: a report is accepted exactly when the page is live.
			_, _, found, err := db.AddReport(ctx, r.PublicID, "spam", "", now, 1000)
			if err != nil {
				t.Fatal(err)
			}
			if found != (r.Visibility == lifecycle.Unlisted && r.Live(now)) {
				t.Errorf("%s at deadline %+v: report accepted %v", c.name, now.Sub(d), found)
			}
		}
	}
}

func published(n int) lifecycle.Record {
	r := record(n)
	r.Visibility = lifecycle.Unlisted
	p := r.CreatedAt
	r.PublishedAt = &p
	return r
}

// TestReports (F-27 R-2 to R-4): stored with reason, message and time; at
// most max per workout; deleted with their row; pruned by filing time.
func TestReports(t *testing.T) {
	db, _ := openTemp(t)
	a, b := published(1), published(2)
	for _, r := range []lifecycle.Record{a, b} {
		if err := db.InsertWorkout(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	now := a.CreatedAt.Add(time.Hour)
	for i := 0; i < 3; i++ {
		id, wid, found, err := db.AddReport(ctx, a.PublicID, "other", "msg", now.Add(time.Duration(i)*time.Second), 3)
		if err != nil || !found || id != int64(i+1) || wid != a.ID {
			t.Fatalf("report %d: %d %q %v %v", i, id, wid, found, err)
		}
	}
	if id, wid, found, err := db.AddReport(ctx, a.PublicID, "spam", "fourth", now, 3); err != nil || !found || id != 0 || wid != a.ID {
		t.Errorf("over the cap: %d %v %v", id, found, err)
	}
	if _, _, _, err := db.AddReport(ctx, b.PublicID, "spam", "", now, 3); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.ReportCount(ctx, a.ID); n != 3 {
		t.Errorf("count %d", n)
	}
	var reason, msg string
	var at int64
	if err := db.db.QueryRowContext(ctx, "SELECT reason, message, created_at FROM reports WHERE id = 1").Scan(&reason, &msg, &at); err != nil || reason != "other" || msg != "msg" || at != now.Unix() {
		t.Errorf("row %q %q %d %v", reason, msg, at, err)
	}
	// The CHECKs back the Go rule up.
	for _, q := range []string{
		"INSERT INTO reports (workout_id, reason, created_at) VALUES ('" + a.ID + "', 'rude', 1)",
		"INSERT INTO reports (workout_id, reason, message, created_at) VALUES ('" + a.ID + "', 'spam', '" + strings.Repeat("ü", 1001) + "', 1)",
		"INSERT INTO reports (workout_id, reason, created_at) VALUES ('" + strings.Repeat("f", 32) + "', 'spam', 1)",
	} {
		if _, err := db.db.ExecContext(ctx, q); err == nil {
			t.Errorf("accepted: %.80s", q)
		}
	}
	if _, err := db.db.ExecContext(ctx, "INSERT INTO reports (workout_id, reason, message, created_at) VALUES (?, 'spam', ?, 1)", b.ID, strings.Repeat("ü", 1000)); err != nil {
		t.Errorf("1000 code points refused: %v", err)
	}

	// Cascade: the row's delete takes its reports, and only its.
	if _, err := db.DeleteWorkout(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int](t, db, "SELECT count(*) FROM reports WHERE workout_id = '"+a.ID+"'"); n != 0 {
		t.Errorf("%d reports outlived their workout", n)
	}
	if n := scalar[int](t, db, "SELECT count(*) FROM reports"); n != 2 {
		t.Errorf("%d reports left, want b's 2", n)
	}

	// Pruning: at or before the cut-off.
	if n, err := db.DeleteReportsFiledBy(ctx, time.Unix(0, 0)); err != nil || n != 0 {
		t.Errorf("pruned %d before 1: %v", n, err)
	}
	if n, err := db.DeleteReportsFiledBy(ctx, time.Unix(1, 0)); err != nil || n != 1 {
		t.Errorf("pruned %d at 1: %v", n, err)
	}
	if n, err := db.DeleteReportsFiledBy(ctx, now.Add(-time.Second)); err != nil || n != 0 {
		t.Errorf("pruned %d younger: %v", n, err)
	}
	if n, err := db.DeleteReportsFiledBy(ctx, now); err != nil || n != 1 {
		t.Errorf("pruned %d at now: %v", n, err)
	}
	// Numbers are never reused, also once the highest ones are gone.
	if id, _, _, err := db.AddReport(ctx, b.PublicID, "spam", "", now, 3); err != nil || id != 6 {
		t.Errorf("number after deletes %d, want 6: %v", id, err)
	}
}

// TestOpenExisting (F-27 R-8): never creates a file, never migrates, and
// refuses any schema version but the binary's.
func TestOpenExisting(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "workouts.db")
	if _, err := OpenExisting(ctx, missing); err != ErrNoDatabase {
		t.Errorf("missing: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("created: %v", err)
	}
	if _, err := OpenExisting(ctx, dir); err != ErrNoDatabase {
		t.Errorf("directory: %v", err)
	}
	db, path := openTemp(t)
	db.Close()
	ok, err := OpenExisting(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	ok.Close()
	for _, v := range []int{3, 5} {
		raw, err := sql.Open("sqlite", "file:"+path)
		if err != nil {
			t.Fatal(err)
		}
		raw.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
		raw.Close()
		if _, err := OpenExisting(ctx, path); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("schema %d is not this binary's (4)", v)) {
			t.Errorf("version %d: %v", v, err)
		}
	}
	raw, _ := sql.Open("sqlite", "file:"+path)
	defer raw.Close()
	var n int
	raw.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'reports'").Scan(&n)
	if n != 1 {
		t.Error("schema changed")
	}
}
