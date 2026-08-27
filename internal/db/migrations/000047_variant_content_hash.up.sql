ALTER TABLE variants ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';
UPDATE variants SET content_hash = lower(hex(randomblob(16))) WHERE content_hash = '';
