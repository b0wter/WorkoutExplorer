-- F-37 DK-2 (ADR-042 items 2, 4): a report's single-use delete key, only as
-- the SHA-256 of its text. Set only when the report's notification is sent,
-- cleared on use and at a start without Pushover; it goes with its report.
ALTER TABLE reports ADD COLUMN delete_key_hash BLOB CHECK (delete_key_hash IS NULL OR length(delete_key_hash) = 32);
CREATE UNIQUE INDEX reports_delete_key_hash ON reports (delete_key_hash) WHERE delete_key_hash IS NOT NULL;
