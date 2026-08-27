ALTER TABLE variants ADD COLUMN thumbnail_key          TEXT;
ALTER TABLE variants ADD COLUMN thumbnail_content_type TEXT NOT NULL DEFAULT 'image/jpeg';
