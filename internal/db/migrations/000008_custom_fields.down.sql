DROP INDEX IF EXISTS idx_pfv_date;
DROP INDEX IF EXISTS idx_pfv_number;
DROP INDEX IF EXISTS idx_pfv_text;
DROP INDEX IF EXISTS idx_pfv_field;
DROP INDEX IF EXISTS idx_pfv_project;
DROP TABLE IF EXISTS project_field_values;

DROP INDEX IF EXISTS idx_afv_boolean;
DROP INDEX IF EXISTS idx_afv_date;
DROP INDEX IF EXISTS idx_afv_number;
DROP INDEX IF EXISTS idx_afv_text;
DROP INDEX IF EXISTS idx_afv_field;
DROP INDEX IF EXISTS idx_afv_asset;
DROP TABLE IF EXISTS asset_field_values;

DROP INDEX IF EXISTS idx_field_defs_active;
DROP INDEX IF EXISTS idx_field_defs_workspace;
DROP TABLE IF EXISTS field_definitions;
