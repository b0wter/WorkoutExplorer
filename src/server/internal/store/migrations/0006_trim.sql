-- F-47 TR-3 (ADR-047 item 2): the trim as two instants (Unix seconds) of the
-- resolved first and last remaining sample; NULL = that end is not trimmed.
ALTER TABLE workouts ADD COLUMN trim_start_at INTEGER;
ALTER TABLE workouts ADD COLUMN trim_end_at INTEGER CHECK (trim_start_at IS NULL OR trim_end_at IS NULL OR trim_start_at < trim_end_at);
