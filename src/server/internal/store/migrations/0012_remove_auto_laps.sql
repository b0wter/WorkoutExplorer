-- F-66 RA-1: whether the owner removed the device's automatic laps (0/1).
-- One value, no coordinates. Goes with the row.
ALTER TABLE workouts ADD COLUMN remove_auto_laps INTEGER NOT NULL DEFAULT 0 CHECK (remove_auto_laps IN (0, 1));
