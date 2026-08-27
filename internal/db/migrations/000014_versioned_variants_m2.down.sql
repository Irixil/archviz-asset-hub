-- VV-M2 down: clear back-filled asset_version_id values.
-- VV-M1 down must run afterwards to drop the column.
UPDATE variants SET asset_version_id = NULL;
