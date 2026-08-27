# Projects & folders

Projects and folders let you organize assets into a structured hierarchy.

## Projects

A **project** is a top-level container, think of it as a client, campaign, or product line.

### Create a project

1. In the sidebar, click **+ New project**
2. Enter a name and optionally a description
3. Press **Create**

Each project has a **color dot** shown in the sidebar and breadcrumbs, making it easy to tell projects apart at a glance.

### Project settings

Open the project info panel (click the `⋮` icon) to rename the project, change its color, add a description, set a cover image, or delete it.

Deleting a project does **not** delete its assets, they are moved to the workspace root (no project).

### Viewing all assets in a project

Click the project name in the sidebar (not a specific folder) to see all assets across all folders in that project at once.

## Folders

Folders live inside projects and can be nested up to **two levels deep**:

```
Project
└── Root folder       ← level 1
    └── Subfolder     ← level 2 (deepest allowed)
```

This covers the vast majority of real workflows. If you find yourself needing a third level, it usually means the work should be split into separate projects.

### Create a folder

Right-click a project in the sidebar and choose **New folder**, or open the project and click **+ Folder** in the header. Right-click an existing folder to create a subfolder.

### Move assets into folders

Drag assets from the grid and drop them onto a folder in the sidebar. You can also select multiple assets and use **Move to folder** from the bulk action bar.

![Projects, folders, and tags in the sidebar](/docs/screenshot_asset_folders_drop.png)

## Moving assets between projects

Select one or more assets, open the bulk action bar, and choose **Move to project**.

## Tags

Tags are workspace-wide labels that can be applied to any asset regardless of project. A single asset can have multiple tags.

### Applying tags

Open any asset's detail panel and type in the tag field. Tags are created on first use, no pre-registration needed. Press `Enter` or comma to confirm each tag.

### Bulk tagging

Select multiple assets and choose **Add tag** from the bulk action bar to apply a tag to all of them at once.

### Removing a tag

Open an asset and click the `×` on any tag chip. For multiple assets, use bulk select and **Remove tag** from the bulk action bar.

### Tag management

![Tag management in Settings](/docs/screenshot_tags_management_dark.png)

Go to **Settings → Tags** to see all workspace tags with their asset counts. From here you can:

- **Rename** a tag (updates all assets instantly)
- **Merge** two tags into one
- **Delete** a tag (removes it from all assets, the assets themselves are unaffected)
