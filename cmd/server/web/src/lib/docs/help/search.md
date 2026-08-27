# Search & filters

Find assets quickly with full-text search and the filter bar.

## Search

Click the search bar or press `S` to focus it. Type any word, Damask searches asset names, tags, project names, and all text-type custom field values.

Results update as you type.

## Filters

![Search with tag and field filters active](/docs/screenshot_search_tags_fields.png)

The filter bar lets you narrow results by:

- **Project**, limit to a specific project
- **Tags**, show only assets with selected tags (multiple tags use AND logic, only assets matching all selected tags appear)
- **File type**, image, video, PDF, etc.
- **Custom fields**, filter by any defined field value (see below)
- **Date uploaded**, a date range picker

All active filters apply together with AND logic, only assets matching every condition are shown. Active filters appear as dismissible chips in the filter bar.

## Custom field filters

Each custom field type has its own filter control:

| Field type | Filter control                                                |
| ---------- | ------------------------------------------------------------- |
| Text / URL | Text input, matches substrings                                |
| Number     | Min/max range inputs                                          |
| Date       | From/to date range picker                                     |
| Yes / No   | Three-state toggle (any / yes / no)                           |
| Select     | Checkbox list of options (OR within field, AND across fields) |

![Custom field filter controls](/docs/screenshot_search_tags_fields_alt.png)

## Combining search and filters

Search and filters work together. You can search for `hero` while also filtering by the `approved` tag, only assets matching both conditions appear.

## Keyboard shortcuts

| Key      | Action                                                                        |
| -------- | ----------------------------------------------------------------------------- |
| `S`      | Focus the search bar from anywhere in the library                             |
| `Ctrl+K` | Open the command palette (also searches assets and navigates to recent items) |

## Sorting

Use the sort controls in the top bar to order results by:

- Date uploaded (newest first by default)
- Name (A-Z)
- File size

## Visual similarity

Click **Find Similar** on any image in the lightbox to find visually similar images across the library.

Two ways to use it:

- **Lightbox drawer**: the results appear inline in the lightbox sidebar
- **Grid view**: click "Open in grid view" to apply the similarity filter to the full library; a banner shows the anchor image name and result count, dismiss it with the X to clear the filter

Notes:

- Images only; the button is hidden for videos, PDFs, and other file types
- Images uploaded before this feature was enabled may not be indexed yet; run the backfill from **Workspace Settings > Content**
- The similarity filter combines with other active filters (tags, project, file type, etc.)
- The URL updates to `?similar_to=<id>` so the filtered view is shareable and bookmarkable
