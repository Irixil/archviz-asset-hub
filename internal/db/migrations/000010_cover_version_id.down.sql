-- SQLite does not support DROP COLUMN on older versions; this migration is
-- intentionally a no-op for the down path. The columns are nullable and
-- harmless if left in place during development rollbacks.
SELECT 1;
