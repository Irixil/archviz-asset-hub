-- SQLite does not support DROP COLUMN on older versions; recreate table without the columns.
-- For simplicity, this down migration is a no-op — the columns default to 0 and are harmless.
SELECT 1;
