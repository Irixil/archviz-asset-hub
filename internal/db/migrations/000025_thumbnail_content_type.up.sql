ALTER TABLE assets
  ADD COLUMN thumbnail_content_type TEXT NOT NULL DEFAULT 'image/jpeg';

ALTER TABLE asset_versions
  ADD COLUMN thumbnail_content_type TEXT NOT NULL DEFAULT 'image/jpeg';
