ALTER TABLE assets ADD COLUMN touched_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00';
CREATE INDEX idx_assets_touched ON assets(touched_at DESC);
