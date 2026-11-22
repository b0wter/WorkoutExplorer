-- F-65 AL-1 (ADR-060): the owner's added lap boundaries of a workout, the
-- whole set as one JSON array of Unix seconds of resolved samples, ascending,
-- no repeats; NULL = none. No coordinates. Goes with the row.
ALTER TABLE workouts ADD COLUMN added_lap_boundaries TEXT CHECK (added_lap_boundaries IS NULL OR json_valid(added_lap_boundaries));
