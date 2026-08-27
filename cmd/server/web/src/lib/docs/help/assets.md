# Assets

Assets are the files at the heart of Damask - images, videos, PDFs, audio, documents, and more.

![Asset detail panel](/docs/screenshot_asset_open.dark.png)

## Uploading

Click **Upload** in the top bar or press `Ctrl+U`. You can upload multiple files at once. Damask processes them in the background and shows upload progress in the status bar.

## Selecting assets

- Click an asset to select it
- `Shift+click` to extend the selection
- Hover over an asset and check the checkbox that appears to add it to a multi-selection
- `Ctrl+A` to select all visible assets
- `Ctrl+Shift+I` to invert the selection
- `Escape` to clear the selection

When multiple assets are selected, the **bulk action bar** appears at the bottom of the screen with options to tag, move, download, or delete them.

## Renaming

Right-click an asset and choose **Rename**, or select it and press `R`.

## Tags

Tags are free-form labels for categorizing assets.

- Open an asset and click **+ Add tag**
- Type a tag name and press Enter
- Tags are shared across the workspace, reuse them freely

To bulk-tag assets, select multiple assets and use **Tag** in the bulk action bar.

## Custom fields

Custom fields let you attach structured metadata to assets (e.g. photographer, license expiry, approval status).

1. Go to **Settings → Custom fields** to define field types (text, number, date, yes/no, select, URL)

![Custom fields settings](/docs/screenshot_custom_fields.dark.png)

2. Open any asset and fill in values in the **Fields** panel

![Tags and custom fields on an asset](/docs/screenshot_asset_tags_fields.dark.png)

Fields can also be scoped to **projects**, useful for fields like `client` or `campaign` that are the same for every asset in a project. Set a value once on the project and new assets inherit it automatically.

To fill in a field across many assets at once, select them and use **Set field** from the bulk action bar.

## Downloading

Click the **Download** button on an asset detail page, or press `Ctrl+D` when an asset is selected.

For automated or scheduled exports to SFTP or Google Drive, see [Exporting](help/exports).

## Deleting

Select one or more assets and press `Ctrl+Backspace`, or use **Delete** from the context menu. Bulk delete requires owner role.

## Version history

Every time you upload a new file to an existing asset, the previous version is preserved. Versions are numbered v1, v2, v3… and numbers are never reused.

### Upload a new version

Open the asset detail panel and click **Upload new version**. You can add an optional comment describing what changed.

### View and restore versions

![Version history tab](/docs/screenshot_asset_versions.dark.png)

Open the **Versions** tab in the asset detail panel. Each version shows its thumbnail, file size, uploader, and upload comment. Click **Restore** on any older version to make it the current one, the version you restore from stays in the history and can be restored again at any time.

### Delete a version

Owners can delete non-current versions from the Versions tab. You cannot delete the current version, restore another version first.

### Version retention

By default all versions are kept indefinitely. Configure a cap (keep last N versions) in **Settings → Workspace → Version history**.

## Activity log

The **Activity** tab on every asset shows a complete history of what changed: renames, tag additions and removals, field value updates, share link creation, downloads, and version changes. Events are newest-first.

![Asset activity log](/docs/screenshot_asset_activity.dark.png)

A workspace-wide recent activity feed is also available from the settings sidebar.

![Workspace-wide activity feed](/docs/screenshot_ws_activity.dark.png)

## Keyboard shortcuts

| Key              | Action                      |
| ---------------- | --------------------------- |
| `Ctrl+U`         | Upload                      |
| `R`              | Rename selected asset       |
| `Ctrl+D`         | Download selected asset     |
| `Ctrl+Backspace` | Delete selected asset(s)    |
| `Ctrl+Shift+S`   | Share selected asset(s)     |
| `Enter`          | Open asset detail panel     |
| `Ctrl+A`         | Select all                  |
| `Ctrl+Shift+I`   | Invert selection            |
| `Escape`         | Clear selection             |
| `V`              | Toggle grid/list layout     |
| `S`              | Focus search bar            |
| `Ctrl+K`         | Open command palette        |
| `?`              | Show all keyboard shortcuts |
| `g l`            | Go to Library               |
| `g t`            | Go to Tags                  |
| `g s`            | Go to Settings              |
| `g h`            | Go to Shares                |
