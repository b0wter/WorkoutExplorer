-- ADR-019 item 8 (rev. 2): no private state (SH-2). SQLite cannot change a
-- CHECK in place, so workouts is rebuilt with visibility in (draft, unlisted).
-- Private rows are not copied, never turned unlisted; their private/{id}/
-- folders go at the next start (ADR-022 recovery).
CREATE TABLE workouts_v3 (
    id                 TEXT    PRIMARY KEY CHECK (length(id) = 32),
    public_id          TEXT    NOT NULL UNIQUE CHECK (length(public_id) = 22),
    edit_token_hash    BLOB    NOT NULL CHECK (length(edit_token_hash) = 32),
    title              TEXT    NOT NULL DEFAULT '',
    description        TEXT    NOT NULL DEFAULT '',
    sport              TEXT    NOT NULL,
    start_time         INTEGER,
    time_zone_offset_s INTEGER,
    visibility         TEXT    NOT NULL CHECK (visibility IN ('draft', 'unlisted')),
    hide_ends          INTEGER NOT NULL DEFAULT 0 CHECK (hide_ends IN (0, 1)),
    allow_download     INTEGER NOT NULL DEFAULT 0 CHECK (allow_download IN (0, 1)),
    summary_json       TEXT    NOT NULL CHECK (json_valid(summary_json)),
    data_end_s         INTEGER CHECK (data_end_s >= 0),
    created_at         INTEGER NOT NULL,
    published_at       INTEGER,
    expires_at         INTEGER NOT NULL,
    hidden1_first      INTEGER CHECK (hidden1_first >= 0),
    hidden1_last       INTEGER
        CHECK ((hidden1_last IS NULL) = (hidden1_first IS NULL) AND hidden1_last >= hidden1_first),
    hidden2_first      INTEGER
        CHECK (hidden2_first IS NULL OR (hidden1_last IS NOT NULL AND hidden2_first > hidden1_last + 1)),
    hidden2_last       INTEGER
        CHECK ((hidden2_last IS NULL) = (hidden2_first IS NULL) AND hidden2_last >= hidden2_first),
    CHECK ((visibility = 'draft') = (published_at IS NULL)),
    CHECK (expires_at > created_at AND expires_at <= created_at + 2592000)
) STRICT;

INSERT INTO workouts_v3 SELECT * FROM workouts WHERE visibility IN ('draft', 'unlisted');

DROP TABLE workouts;
ALTER TABLE workouts_v3 RENAME TO workouts;
CREATE INDEX workouts_expires_at ON workouts (expires_at);
