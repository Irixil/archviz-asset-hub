-- Adds workspace-level OpenRouter API key (AES-256-GCM encrypted).
-- NULL means no workspace override; env key is used as fallback.
ALTER TABLE workspaces ADD COLUMN openrouter_api_key_enc TEXT;
