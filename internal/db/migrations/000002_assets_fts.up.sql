CREATE VIRTUAL TABLE IF NOT EXISTS assets_fts USING fts5(
    original_filename,
    content='assets',
    content_rowid='rowid'
);

INSERT INTO assets_fts(rowid, original_filename)
SELECT rowid, original_filename FROM assets;

CREATE TRIGGER assets_fts_ai AFTER INSERT ON assets BEGIN
    INSERT INTO assets_fts(rowid, original_filename) VALUES (new.rowid, new.original_filename);
END;

CREATE TRIGGER assets_fts_ad AFTER DELETE ON assets BEGIN
    INSERT INTO assets_fts(assets_fts, rowid, original_filename) VALUES ('delete', old.rowid, old.original_filename);
END;

CREATE TRIGGER assets_fts_au AFTER UPDATE ON assets BEGIN
    INSERT INTO assets_fts(assets_fts, rowid, original_filename) VALUES ('delete', old.rowid, old.original_filename);
    INSERT INTO assets_fts(rowid, original_filename) VALUES (new.rowid, new.original_filename);
END;
