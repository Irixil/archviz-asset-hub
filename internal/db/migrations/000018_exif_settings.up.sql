ALTER TABLE workspaces ADD COLUMN exif_keep INTEGER NOT NULL DEFAULT 0;
-- 0 = don't extract EXIF on upload (default)
-- 1 = extract EXIF on upload

ALTER TABLE workspaces ADD COLUMN exif_keep_gps INTEGER NOT NULL DEFAULT 0;
-- 0 = strip GPS coordinates (default, privacy-safe)
-- 1 = retain GPS in field values
