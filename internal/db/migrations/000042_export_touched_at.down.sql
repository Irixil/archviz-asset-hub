DROP INDEX IF EXISTS idx_assets_touched;
-- SQLite does not support DROP COLUMN in older versions; this migration is irreversible
-- on SQLite < 3.35.0. Assets table retains the touched_at column.
