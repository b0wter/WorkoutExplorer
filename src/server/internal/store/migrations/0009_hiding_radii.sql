-- F-55 HR-1 (ADR-056): a hiding radius per end, in metres; 0 = that end is
-- not cut. Hiding on becomes 300 m at both ends, off 0 at both. hide_ends and
-- ADR-030's four range columns go; hiding_steps (JSON, display only) is NULL
-- until the next regenerate. The range columns' CHECKs reference only each
-- other, so dropping them last-first needs no table rebuild.
ALTER TABLE workouts ADD COLUMN hide_start_m INTEGER NOT NULL DEFAULT 0
    CHECK (hide_start_m BETWEEN 0 AND 2000 AND hide_start_m % 50 = 0);
ALTER TABLE workouts ADD COLUMN hide_end_m INTEGER NOT NULL DEFAULT 0
    CHECK (hide_end_m BETWEEN 0 AND 2000 AND hide_end_m % 50 = 0);
UPDATE workouts SET hide_start_m = 300, hide_end_m = 300 WHERE hide_ends = 1;
ALTER TABLE workouts DROP COLUMN hidden2_last;
ALTER TABLE workouts DROP COLUMN hidden2_first;
ALTER TABLE workouts DROP COLUMN hidden1_last;
ALTER TABLE workouts DROP COLUMN hidden1_first;
ALTER TABLE workouts DROP COLUMN hide_ends;
ALTER TABLE workouts ADD COLUMN hiding_steps TEXT CHECK (hiding_steps IS NULL OR json_valid(hiding_steps));
