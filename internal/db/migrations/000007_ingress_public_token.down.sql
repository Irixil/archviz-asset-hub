DROP INDEX IF EXISTS idx_ingress_sources_public_token;
ALTER TABLE ingress_sources DROP COLUMN public_token;
