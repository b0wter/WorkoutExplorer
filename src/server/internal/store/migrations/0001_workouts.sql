CREATE TABLE workouts (
    id                 TEXT    PRIMARY KEY CHECK (length(id) = 32),
    public_id          TEXT    NOT NULL UNIQUE CHECK (length(public_id) = 22),
    edit_token_hash    BLOB    NOT NULL CHECK (length(edit_token_hash) = 32),
    title              TEXT    NOT NULL DEFAULT '',
    description        TEXT    NOT NULL DEFAULT '',
    sport              TEXT    NOT NULL,
    start_time         INTEGER,
    time_zone_offset_s INTEGER,
    visibility         TEXT    NOT NULL CHECK (visibility IN ('draft', 'unlisted', 'private')),
    hide_ends          INTEGER NOT NULL DEFAULT 0 CHECK (hide_ends IN (0, 1)),
    allow_download     INTEGER NOT NULL DEFAULT 0 CHECK (allow_download IN (0, 1)),
    summary_json       TEXT    NOT NULL CHECK (json_valid(summary_json)),
    data_end_s         INTEGER CHECK (data_end_s >= 0),
    created_at         INTEGER NOT NULL,
    published_at       INTEGER,
    expires_at         INTEGER NOT NULL,
    CHECK ((visibility = 'draft') = (published_at IS NULL)),
    CHECK (expires_at > created_at AND expires_at <= created_at + 2592000)
) STRICT;

CREATE INDEX workouts_expires_at ON workouts (expires_at);
