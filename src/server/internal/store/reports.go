package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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
