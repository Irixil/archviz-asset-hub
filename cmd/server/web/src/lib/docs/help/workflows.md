# Workflows

Workflows automate actions on your assets. Each workflow is a graph of nodes, a trigger that starts the run, optional filters that route assets based on their properties, and actions that do the work. Workflows run in the background as jobs; you can inspect every run and see exactly which step succeeded or failed.

Go to **Settings → Workflows** to create and manage workflows.

![Workflows list](/docs/screenshot_workflows_list.png)

## How a workflow runs

When a trigger event fires (e.g. an asset is uploaded), Damask finds all enabled workflows whose trigger type matches that event. Each matching workflow starts a run. The run walks the graph node by node: filter nodes route the asset down a **Match** or **No match** branch; action nodes perform an operation and then continue down the **Out** port. If any node fails it exits through an **Error** port and the run is marked failed.

You can also run any workflow manually from the workflow detail page, or trigger it against a specific set of assets using the bulk-run option.

## Triggers

Each workflow has exactly one trigger node. The trigger determines what starts the run.

| Trigger              | When it fires                                         | Config                                          |
| -------------------- | ----------------------------------------------------- | ----------------------------------------------- |
| **Manual**           | Only when you click Run or use the API                | ,                                               |
| **Asset Created**    | An asset is uploaded to the workspace                 | Optional: restrict to a specific project/folder |
| **Version Uploaded** | A new version is uploaded to an existing asset        | Optional: restrict to a specific asset          |
| **Tag Added**        | A tag is added to an asset                            | Required: tag name to watch                     |
| **Schedule**         | On a cron schedule                                    | Required: cron expression                       |
| **Webhook**          | An inbound HTTP request to the workflow's webhook URL | ,                                               |

## Filters

Filters let you route an asset down different branches based on its properties. Each filter has two output ports: **Match** (condition met) and **No match** (condition not met). Connect each port to the next node, or leave one unconnected to stop that branch silently.

| Filter                | Routes on                                         | Config                                               |
| --------------------- | ------------------------------------------------- | ---------------------------------------------------- |
| **Filter MIME Type**  | Whether the asset's MIME type starts with a value | `prefix`, e.g. `image/`, `video/`, `application/pdf` |
| **Filter Filename**   | Filename substring or extension                   | `contains` and/or `extension` (e.g. `.pdf`)          |
| **Filter Size**       | File size in bytes                                | `min` and/or `max` bytes                             |
| **Filter Tag**        | Whether the asset has a specific tag              | `name`, tag to check for                             |
| **Filter Folder**     | Whether the asset is in a specific folder         | `folder_id`                                          |
| **Filter Expression** | A key/value comparison against the run context    | `key` and `value`                                    |

## Actions

Actions are the nodes that do the work. Each action has an **Out** port (success) and an **Error** port (failure).

| Action              | What it does                                                | Key config                                                               |
| ------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------ |
| **Create Variant**  | Queues a variant processing job for the asset               | `type` (variant type), `params` (variant-specific settings), `title`     |
| **Create Share**    | Creates a share link for the asset                          | `label`, `allow_comments`, `allow_download`, `expires_in_days`           |
| **Tag Asset**       | Adds a tag to the asset                                     | `name`, tag to add                                                       |
| **Move Asset**      | Moves the asset to a different folder or project            | `folder_id` and/or `project_id`                                          |
| **Set Asset Field** | Sets a custom field value on the asset                      | `field_id`, `value`                                                      |
| **Fan Out**         | Forwards execution to every connected branch simultaneously | , (used to run multiple actions in parallel from a single upstream node) |

## Inspect a run

Workflows runs can be reviewed and inspected using a read only node graph.

![Workflows editor](/docs/screenshot_workflows_inspect.png)

## Editor

Workflows are created and edited using a node-based graphical editor.

![Workflows editor](/docs/screenshot_workflows_editor.png)

## Run history

Every workflow run is recorded. Open a workflow and click **Runs** to see the history. Each run shows:

- **Status**, `pending`, `running`, `completed`, or `failed`
- **Trigger data**, the event payload that started the run
- **Step trace**, each node that executed, its output port, and any error message

If a run fails and **Notify on failure** is enabled on the workflow, Damask sends an email to the workspace owner.

- If a run fails multiple times in a row, Damask disable it, and sends an email to the workspace owner.

![Workflows list](/docs/screenshot_workflows_list_history.png)

## Templates

When creating a new workflow, you can start from a template instead of a blank canvas:

| Template                   | What it does                                                          |
| -------------------------- | --------------------------------------------------------------------- |
| **Image resize on upload** | Filters for `image/` MIME type, then creates a 1600×1600 fit resize   |
| **Move PDFs to folder**    | Filters for `application/pdf`, then moves matching assets to a folder |
| **Share on upload**        | Creates a share link for every uploaded asset                         |
| **Tag video uploads**      | Filters for `video/` MIME type on version upload, then adds a tag     |
| **Blank (manual trigger)** | A single manual trigger node, start from scratch                      |

## Example: auto-resize every uploaded image

1. Go to **Settings → Workflows** and click **New workflow**.
2. Choose the **Image resize on upload** template (or build manually).
3. The graph has three nodes:
   - **Asset Created** trigger, fires on every upload in the workspace (optionally restrict to a project).
   - **Filter MIME Type**, `prefix: image/`, only images pass through the Match port.
   - **Create Variant**, type `image_resize`, params `{ "width": 1600, "height": 1600, "mode": "fit" }`.
4. Name the workflow, toggle it **Enabled**, and save.

Every image uploaded to the workspace now gets a 1600×1600 fit resize variant queued automatically. Video and other file types hit the **No match** port and are ignored.
