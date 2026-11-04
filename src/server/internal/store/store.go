// Package store is the SQLite adapter of the workouts table (ADR-019). It is
// the only importer of the database driver. SQLite holds ownership and
// settings only, never streams or laps.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver "sqlite"

	"workout-explorer/internal/lifecycle"
)

// DB is the open database. One connection serialises every statement
// (ADR-019 item 4).
type DB struct{ db *sql.DB }

// Open creates the file at path with mode 0600 if missing, opens it in WAL
// mode with secure_delete (a deleted row is zeroed, not left in a free page;
// QA-21-r1-1, AC-4), checks that WAL is on, and applies the migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	return open(ctx, path, migrate)
}

// ErrNoDatabase: OpenExisting found no file at the path.
var ErrNoDatabase = errors.New("store: no database file")

// OpenExisting opens the database at path for an operator command (F-27
// R-8): the file must exist (it is never created) and its schema version
// must be this binary's; nothing is migrated, since serve migrates at start.
func OpenExisting(ctx context.Context, path string) (*DB, error) {
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil, ErrNoDatabase
	}
	return open(ctx, path, sameVersion)
}

func open(ctx context.Context, path string, schema func(context.Context, *sql.DB) error) (*DB, error) {

	u := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath()}
	q := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "secure_delete(ON)", "synchronous(NORMAL)", "foreign_keys(ON)", "busy_timeout(5000)", "temp_store(MEMORY)"} {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")
	dsn := u.String() + "?" + strings.ReplaceAll(q.Encode(), "+", "%20")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*DB, error) {
		db.Close()
		return nil, err
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return fail(fmt.Errorf("store: %w", err))
	}
	if !strings.EqualFold(mode, "wal") {
		return fail(fmt.Errorf("store: journal mode is %q, want wal", mode))
	}
	if err := schema(ctx, db); err != nil {
		return fail(err)
	}
	return &DB{db}, nil
}

// Checkpoint copies the WAL into the database file and truncates it, so the
// frames of a deleted row leave the disk (QA-21-r1-1). It fails when a reader
// keeps it from completing; the caller retries later.
func (d *DB) Checkpoint(ctx context.Context) error {
	var busy, log, done int
	if err := d.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &done); err != nil {
		return fmt.Errorf("store: checkpoint: %w", err)
	}
	if busy != 0 {
		return errors.New("store: checkpoint: busy")
	}
	return nil
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

func unix(t time.Time) int64 { return t.Unix() }

func unixPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func intPtr(p *int) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

// InsertWorkout inserts one row in one transaction.
func (d *DB) InsertWorkout(ctx context.Context, r lifecycle.Record) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO workouts
		(id, public_id, edit_token_hash, title, description, sport, start_time, time_zone_offset_s,
		 visibility, hide_ends, allow_download, summary_json, data_end_s, created_at, published_at, expires_at,
		 hidden1_first, hidden1_last, hidden2_first, hidden2_last)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append([]any{r.ID, r.PublicID, r.EditTokenHash[:], r.Title, r.Description, r.Sport, unixPtr(r.StartTime), intPtr(r.TimeZoneOffsetS),
			string(r.Visibility), r.HideEnds, r.AllowDownload, r.SummaryJSON, intPtr(r.DataEndS),
			unix(r.CreatedAt), unixPtr(r.PublishedAt), unix(r.ExpiresAt)}, rangeColumns(r.HiddenRanges)...)...)
	if err != nil {
		return fmt.Errorf("store: insert workout: %w", err)
	}
	return nil
}

// WorkoutIDs returns the internal IDs of every row.
func (d *DB) WorkoutIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT id FROM workouts")
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	return out, nil
}

// LivePublicIDs returns the public_id of every unlisted row that expires
// after now (F-15 P-8: the folders allowed under public/). For an unlisted
// row the deadline is expires_at, so this is the SQL form of
// lifecycle.Record.Live (F-21 X-2; TestDeadlineBoundary pins the two).
func (d *DB) LivePublicIDs(ctx context.Context, now time.Time) (map[string]struct{}, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT public_id FROM workouts WHERE visibility = 'unlisted' AND expires_at > ?", unix(now))
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	return out, nil
}

// DueWorkouts returns every row past its deadline at now (F-21 X-1, X-2):
// expires_at reached, or a draft DraftLifetime after created_at. It is the
// SQL form of lifecycle.Record.Live; TestDeadlineBoundary pins the two.
func (d *DB) DueWorkouts(ctx context.Context, now time.Time) ([]lifecycle.Record, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT "+columns+` FROM workouts
		WHERE expires_at <= ?1 OR (visibility = 'draft' AND created_at <= ?1 - ?2)`,
		unix(now), int64(lifecycle.DraftLifetime/time.Second))
	if err != nil {
		return nil, fmt.Errorf("store: due workouts: %w", err)
	}
	defer rows.Close()
	var out []lifecycle.Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("store: due workouts: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: due workouts: %w", err)
	}
	return out, nil
}

// WorkoutExists reports whether a row with this internal ID exists.
func (d *DB) WorkoutExists(ctx context.Context, id string) (bool, error) {
	var one int
	err := d.db.QueryRowContext(ctx, "SELECT 1 FROM workouts WHERE id = ?", id).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("store: %w", err)
	}
	return true, nil
}

// DeleteWorkout deletes the row with this internal ID, if any. deleted is
// true only for the call that removed the row; an absent row is false and nil
// (F-10: only that call releases the workout's bytes).
func (d *DB) DeleteWorkout(ctx context.Context, id string) (deleted bool, err error) {
	res, err := d.db.ExecContext(ctx, "DELETE FROM workouts WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("store: delete workout: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: delete workout: %w", err)
	}
	return n > 0, nil
}

const columns = `id, public_id, edit_token_hash, title, description, sport, start_time, time_zone_offset_s,
	visibility, hide_ends, allow_download, summary_json, data_end_s, created_at, published_at, expires_at,
	hidden1_first, hidden1_last, hidden2_first, hidden2_last`

// rangeColumns maps up to two ranges to the four hidden* columns (ADR-030);
// unused pairs are NULL. More than two ranges cannot come from
// workout.HiddenRanges; the CHECKs refuse anything malformed.
func rangeColumns(r [][2]int) []any {
	out := []any{nil, nil, nil, nil}
	for i := 0; i < len(r) && i < 2; i++ {
		out[2*i], out[2*i+1] = int64(r[i][0]), int64(r[i][1])
	}
	return out
}

type scanner interface{ Scan(dest ...any) error }

func scanRecord(sc scanner) (lifecycle.Record, error) {
	var (
		r                  lifecycle.Record
		hash               []byte
		vis                string
		start, off, end, p sql.NullInt64
		created, expires   int64
		h                  [4]sql.NullInt64
	)
	if err := sc.Scan(&r.ID, &r.PublicID, &hash, &r.Title, &r.Description, &r.Sport, &start, &off,
		&vis, &r.HideEnds, &r.AllowDownload, &r.SummaryJSON, &end, &created, &p, &expires,
		&h[0], &h[1], &h[2], &h[3]); err != nil {
		return lifecycle.Record{}, err
	}
	for i := 0; i < 4 && h[i].Valid && h[i+1].Valid; i += 2 {
		r.HiddenRanges = append(r.HiddenRanges, [2]int{int(h[i].Int64), int(h[i+1].Int64)})
	}
	if len(hash) != len(r.EditTokenHash) {
		return lifecycle.Record{}, errors.New("edit token hash has the wrong length")
	}
	copy(r.EditTokenHash[:], hash)
	r.Visibility = lifecycle.Visibility(vis)
	ts := func(n int64) time.Time { return time.Unix(n, 0).UTC() }
	if start.Valid {
		t := ts(start.Int64)
		r.StartTime = &t
	}
	if off.Valid {
		v := int(off.Int64)
		r.TimeZoneOffsetS = &v
	}
	if end.Valid {
		v := int(end.Int64)
		r.DataEndS = &v
	}
	if p.Valid {
		t := ts(p.Int64)
		r.PublishedAt = &t
	}
	r.CreatedAt, r.ExpiresAt = ts(created), ts(expires)
	return r, nil
}

// WorkoutByPublicID returns the row with this public ID, if any.
func (d *DB) WorkoutByPublicID(ctx context.Context, publicID string) (lifecycle.Record, bool, error) {
	r, err := scanRecord(d.db.QueryRowContext(ctx, "SELECT "+columns+" FROM workouts WHERE public_id = ?", publicID))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return lifecycle.Record{}, false, nil
	case err != nil:
		return lifecycle.Record{}, false, fmt.Errorf("store: %w", err)
	}
	return r, true, nil
}

// UpdateWorkout is a read-modify-write of the settings columns in one
// immediate transaction (F-12 O-3). The connection's transaction mode is
// immediate (see Open), so a writer in another process waits for it.
func (d *DB) UpdateWorkout(ctx context.Context, id string, apply func(lifecycle.Record) (lifecycle.Record, error)) (lifecycle.Record, bool, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return lifecycle.Record{}, false, fmt.Errorf("store: update workout: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	cur, err := scanRecord(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM workouts WHERE id = ?", id))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return lifecycle.Record{}, false, nil
	case err != nil:
		return lifecycle.Record{}, false, fmt.Errorf("store: update workout: %w", err)
	}
	next, err := apply(cur)
	if err != nil {
		return lifecycle.Record{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workouts SET title = ?, description = ?, visibility = ?,
		hide_ends = ?, allow_download = ?, published_at = ?, expires_at = ? WHERE id = ?`,
		next.Title, next.Description, string(next.Visibility), next.HideEnds, next.AllowDownload,
		unixPtr(next.PublishedAt), unix(next.ExpiresAt), id); err != nil {
		return lifecycle.Record{}, false, fmt.Errorf("store: update workout: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return lifecycle.Record{}, false, fmt.Errorf("store: update workout: %w", err)
	}
	return next, true, nil
}

// liveWhere is the SQL form of lifecycle.Record.Live at ?1 with ?2 =
// lifecycle.DraftLifetime in seconds: the negation of DueWorkouts' condition.
const liveWhere = `expires_at > ?1 AND NOT (visibility = 'draft' AND created_at <= ?1 - ?2)`

// LiveWorkouts returns every row that is Live at now, drafts included, oldest
// first (F-30 RG-2).
func (d *DB) LiveWorkouts(ctx context.Context, now time.Time) ([]lifecycle.Record, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT "+columns+" FROM workouts WHERE "+liveWhere+" ORDER BY created_at, id",
		unix(now), int64(lifecycle.DraftLifetime/time.Second))
	if err != nil {
		return nil, fmt.Errorf("store: live workouts: %w", err)
	}
	defer rows.Close()
	var out []lifecycle.Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("store: live workouts: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: live workouts: %w", err)
	}
	return out, nil
}

// UpdateDerived writes the columns derived from the original (sport,
// start_time, time_zone_offset_s, summary_json, data_end_s, the hidden
// ranges) from r, in one statement that changes nothing when the row is gone
// or not Live at now, and returns the row as it is afterwards (F-30 RG-4
// (b)). Settings columns are never written.
func (d *DB) UpdateDerived(ctx context.Context, id string, r lifecycle.Record, now time.Time) (lifecycle.Record, bool, error) {
	got, err := scanRecord(d.db.QueryRowContext(ctx, `UPDATE workouts SET sport = ?3, start_time = ?4, time_zone_offset_s = ?5,
		summary_json = ?6, data_end_s = ?7, hidden1_first = ?8, hidden1_last = ?9, hidden2_first = ?10, hidden2_last = ?11
		WHERE id = ?12 AND `+liveWhere+` RETURNING `+columns,
		append([]any{unix(now), int64(lifecycle.DraftLifetime / time.Second), r.Sport, unixPtr(r.StartTime), intPtr(r.TimeZoneOffsetS),
			r.SummaryJSON, intPtr(r.DataEndS)}, append(rangeColumns(r.HiddenRanges), id)...)...))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return lifecycle.Record{}, false, nil
	case err != nil:
		return lifecycle.Record{}, false, fmt.Errorf("store: update derived: %w", err)
	}
	return got, true, nil
}
