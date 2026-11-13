-- F-49 ZN-4: the sharer's zone values; NULL = use the file's values.
ALTER TABLE workouts ADD COLUMN zones_max_heart_rate INTEGER CHECK (zones_max_heart_rate BETWEEN 100 AND 250);
ALTER TABLE workouts ADD COLUMN zones_threshold_power INTEGER CHECK (zones_threshold_power BETWEEN 50 AND 1000);
