-- F-27 R-2, R-3 (ADR-019 rev. 4 items 6, 10): reports from viewers. Only the
-- reason, the message and the time; never anything about the request. Each
-- report goes with its workout's row (ON DELETE CASCADE), at the latest 30
-- days after it was filed (serve's expiry pass). AUTOINCREMENT: a report's
-- number is never given out again (ADR-039 item 2 names it in notifications).
CREATE TABLE reports (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id TEXT    NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
    reason     TEXT    NOT NULL CHECK (reason IN ('spam', 'illegal_content', 'personal_data', 'other')),
    message    TEXT    NOT NULL DEFAULT '' CHECK (length(message) <= 1000),
    created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX reports_workout_id ON reports (workout_id);
