-- F-56 PN-1 (ADR-057): the map pins of a workout, the whole set as one JSON
-- array in canonical order (sample instants, texts, type); NULL = none. No
-- coordinates. Goes with the row.
ALTER TABLE workouts ADD COLUMN pins TEXT CHECK (pins IS NULL OR json_valid(pins));
