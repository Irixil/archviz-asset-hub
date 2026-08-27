# Audit features

## Activity log export

The workspace activity log tracks every significant event: renames, tag changes, field updates, share link creation, downloads, and version changes.

To export it as a CSV go to **Settings → Activity** and click **Export CSV**. You can optionally filter by date range before exporting.

The CSV includes these columns: `event_id`, `event_type`, `actor_type`, `actor_id`, `actor_name`, `payload`, `created_at`, `human_readable`, `entity_type`, `entity_id`. Rows are newest-first. Exports are capped at 10 000 events.
