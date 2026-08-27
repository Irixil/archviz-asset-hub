ALTER TABLE users ADD COLUMN oidc_sub       TEXT;
ALTER TABLE users ADD COLUMN oidc_issuer    TEXT;
ALTER TABLE users ADD COLUMN canva_user_id  TEXT;
ALTER TABLE users ADD COLUMN google_user_id TEXT;
ALTER TABLE users ADD COLUMN avatar_url     TEXT;
ALTER TABLE users ADD COLUMN auth_methods   TEXT NOT NULL DEFAULT '["password"]';

CREATE UNIQUE INDEX idx_users_oidc   ON users(oidc_issuer, oidc_sub)   WHERE oidc_sub IS NOT NULL;
CREATE UNIQUE INDEX idx_users_google ON users(google_user_id)          WHERE google_user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_users_canva  ON users(canva_user_id)           WHERE canva_user_id IS NOT NULL;
