UPDATE workflows
SET enabled = 0,
    updated_at = datetime('now')
WHERE enabled = 1;
