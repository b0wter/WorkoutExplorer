package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// migrations returns the embedded files in order. Names are NNNN_slug.sql,
// numbered from 0001 without gaps (ADR-019 item 5).
func migrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		num, _, ok := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil || len(num) != 4 {
			return nil, fmt.Errorf("store: migration file %q is not named NNNN_slug.sql", e.Name())
		}
		b, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{v, e.Name(), string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i, m := range out {
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration numbers have a gap before %s", m.name)
		}
	}
	return out, nil
}

// migrate applies every migration above the database's user_version, each in
// its own immediate transaction that also sets user_version. A database
// newer than this binary is refused.
func migrate(ctx context.Context, db *sql.DB) error {
	ms, err := migrations()
	if err != nil {
		return err
	}
	var cur int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}
	if cur > len(ms) {
		return fmt.Errorf("store: database schema %d is newer than this binary (%d)", cur, len(ms))
	}
	for _, m := range ms[cur:] {
		tx, err := db.BeginTx(ctx, nil) // _txlock=immediate: BEGIN IMMEDIATE
		if err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
	}
	return nil
}

// sameVersion refuses a database whose schema version is not this binary's.
func sameVersion(ctx context.Context, db *sql.DB) error {
	ms, err := migrations()
	if err != nil {
		return err
	}
	var cur int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}
	if cur != len(ms) {
		return fmt.Errorf("store: database schema %d is not this binary's (%d); start serve with this binary first", cur, len(ms))
	}
	return nil
}
