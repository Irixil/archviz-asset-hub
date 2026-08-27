-- SQLite ≥3.35 supports DROP COLUMN
ALTER TABLE variants DROP COLUMN thumbnail_content_type;
ALTER TABLE variants DROP COLUMN thumbnail_key;
