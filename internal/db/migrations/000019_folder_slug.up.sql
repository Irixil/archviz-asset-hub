ALTER TABLE folders ADD COLUMN slug TEXT;

UPDATE folders
SET slug = lower(
    replace(
        replace(
            replace(name, ' ', '-'),
        '/', '-'),
    '_', '-')
);

CREATE INDEX idx_folders_workspace_slug ON folders(workspace_id, slug);
