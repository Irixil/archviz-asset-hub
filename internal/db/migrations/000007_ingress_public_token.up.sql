ALTER TABLE ingress_sources ADD COLUMN public_token TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_ingress_sources_public_token
    ON ingress_sources(public_token) WHERE public_token != '';
