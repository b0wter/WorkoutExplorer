-- ADR-030: the sample ranges start and end hiding would remove, as up to two
-- inclusive [first, last] pairs into draft.bin. All NULL: no ranges (no
-- positions, or a row from before this migration). Display only: the public
-- build never reads them.
ALTER TABLE workouts ADD COLUMN hidden1_first INTEGER CHECK (hidden1_first >= 0);
ALTER TABLE workouts ADD COLUMN hidden1_last INTEGER
    CHECK ((hidden1_last IS NULL) = (hidden1_first IS NULL) AND hidden1_last >= hidden1_first);
ALTER TABLE workouts ADD COLUMN hidden2_first INTEGER
    CHECK (hidden2_first IS NULL OR (hidden1_last IS NOT NULL AND hidden2_first > hidden1_last + 1));
ALTER TABLE workouts ADD COLUMN hidden2_last INTEGER
    CHECK ((hidden2_last IS NULL) = (hidden2_first IS NULL) AND hidden2_last >= hidden2_first);
