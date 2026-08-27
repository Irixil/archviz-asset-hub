-- SQLite ≥3.35 supports DROP COLUMN
ALTER TABLE assets DROP COLUMN thumbnail_content_type;
ALTER TABLE asset_versions DROP COLUMN thumbnail_content_type;
