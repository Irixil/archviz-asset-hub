DROP INDEX IF EXISTS idx_users_oidc;
DROP INDEX IF EXISTS idx_users_google;
DROP INDEX IF EXISTS idx_users_canva;

-- SQLite does not support DROP COLUMN before 3.35.0; these columns are left in place on downgrade.
