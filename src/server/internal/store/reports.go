package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"workout-explorer/internal/reports"
)

// The reports table (F-27 R-2 to R-5). A report row holds the reason, the
// message, the time and its workout's internal ID; it goes with that row
// (ON DELETE CASCADE).

// AddReport stores a report on the workout with this public ID in one
// immediate transaction: the workout must be unlisted and live at now (the
// SQL form of lifecycle.Record.Live, as LivePublicIDs; TestDeadlineBoundary
// pins it), else found is false. With max reports already stored, nothing is
// stored and id is 0 (R-4). workoutID is the row's internal ID.
func (d *DB) AddReport(ctx context.Context, publicID, reason, message string, now time.Time, max int) (id int64, workoutID string, found bool, err error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	err = tx.QueryRowContext(ctx, "SELECT id FROM workouts WHERE public_id = ? AND visibility = 'unlisted' AND expires_at > ?",
		publicID, unix(now)).Scan(&workoutID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, "", false, nil
	case err != nil:
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM reports WHERE workout_id = ?", workoutID).Scan(&n); err != nil {
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	if n >= max {
		return 0, workoutID, true, nil
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO reports (workout_id, reason, message, created_at) VALUES (?, ?, ?, ?)",
		workoutID, reason, message, unix(now))
	if err != nil {
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	if id, err = res.LastInsertId(); err != nil {
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", false, fmt.Errorf("store: add report: %w", err)
	}
	return id, workoutID, true, nil
}

// DeleteReportsFiledBy deletes every report filed at or before t (R-3's
// 30-day ceiling) and returns how many.
func (d *DB) DeleteReportsFiledBy(ctx context.Context, t time.Time) (int64, error) {
	res, err := d.db.ExecContext(ctx, "DELETE FROM reports WHERE created_at <= ?", unix(t))
	if err != nil {
		return 0, fmt.Errorf("store: delete reports: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("store: delete reports: %w", err)
	}
	return n, nil
}

// ReportCount is the number of reports on the workout with this internal ID.
func (d *DB) ReportCount(ctx context.Context, workoutID string) (int, error) {
	var n int
	if err := d.db.QueryRowContext(ctx, "SELECT count(*) FROM reports WHERE workout_id = ?", workoutID).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count reports: %w", err)
	}
	return n, nil
}

// Delete keys (F-37 DK-2 to DK-8, ADR-042). Only the key's SHA-256 is ever
// a parameter here; no error text carries it (bound, never formatted in).

// SetDeleteKeyHash stores the hash of the delete key issued for this report
// (DK-3); false if the report is gone.
func (d *DB) SetDeleteKeyHash(ctx context.Context, reportID int64, hash [32]byte) (bool, error) {
	res, err := d.db.ExecContext(ctx, "UPDATE reports SET delete_key_hash = ? WHERE id = ?", hash[:], reportID)
	if err != nil {
		return false, fmt.Errorf("store: set delete key: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: set delete key: %w", err)
	}
	return n == 1, nil
}

// ClearDeleteKeys removes every delete key (DK-5) and returns how many.
func (d *DB) ClearDeleteKeys(ctx context.Context) (int64, error) {
	res, err := d.db.ExecContext(ctx, "UPDATE reports SET delete_key_hash = NULL WHERE delete_key_hash IS NOT NULL")
	if err != nil {
		return 0, fmt.Errorf("store: clear delete keys: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("store: clear delete keys: %w", err)
	}
	return n, nil
}

// keyValid is DK-4 in SQL over the reports row named r: the hash matches,
// the report was filed after filedAfter, and its workout's row lives at now.
func keyValid(r string) string {
	return r + `.delete_key_hash = ? AND ` + r + `.created_at > ?
	AND EXISTS (SELECT 1 FROM workouts w WHERE w.id = ` + r + `.workout_id AND w.expires_at > ?)`
}

// ReportByDeleteKey is the report a valid key points at and its workout's
// row (title, sport, start, zone, expiry only), with the number of reports
// on that workout (DK-7). It changes nothing.
func (d *DB) ReportByDeleteKey(ctx context.Context, hash [32]byte, filedAfter, now time.Time) (reports.KeyedReport, bool, error) {
	var (
		k          reports.KeyedReport
		filed, exp int64
		start, off sql.NullInt64
	)
	err := d.db.QueryRowContext(ctx, `SELECT r.id, r.reason, r.created_at,
			w.title, w.sport, w.start_time, w.time_zone_offset_s, w.expires_at,
			(SELECT count(*) FROM reports c WHERE c.workout_id = r.workout_id)
		FROM reports r JOIN workouts w ON w.id = r.workout_id WHERE `+keyValid("r"),
		hash[:], unix(filedAfter), unix(now)).Scan(&k.ID, &k.Reason, &filed,
		&k.Workout.Title, &k.Workout.Sport, &start, &off, &exp, &k.Count)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return reports.KeyedReport{}, false, nil
	case err != nil:
		return reports.KeyedReport{}, false, fmt.Errorf("store: report by delete key: %w", err)
	}
	k.FiledAt, k.Workout.ExpiresAt = time.Unix(filed, 0).UTC(), time.Unix(exp, 0).UTC()
	if start.Valid {
		t := time.Unix(start.Int64, 0).UTC()
		k.Workout.StartTime = &t
	}
	if off.Valid {
		v := int(off.Int64)
		k.Workout.TimeZoneOffsetS = &v
	}
	return k, true, nil
}

// ConsumeDeleteKey spends a valid key in one statement (DK-8): its hash is
// cleared and the workout's internal and public ID returned. Of concurrent
// calls with one key, also across processes, exactly one gets ok.
func (d *DB) ConsumeDeleteKey(ctx context.Context, hash [32]byte, filedAfter, now time.Time) (id, publicID string, ok bool, err error) {
	// No alias: SQLite does not resolve an UPDATE alias inside RETURNING.
	err = d.db.QueryRowContext(ctx, `UPDATE reports SET delete_key_hash = NULL WHERE `+keyValid("reports")+`
		RETURNING workout_id, (SELECT w.public_id FROM workouts w WHERE w.id = reports.workout_id)`,
		hash[:], unix(filedAfter), unix(now)).Scan(&id, &publicID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", "", false, nil
	case err != nil:
		return "", "", false, fmt.Errorf("store: consume delete key: %w", err)
	}
	return id, publicID, true, nil
}
