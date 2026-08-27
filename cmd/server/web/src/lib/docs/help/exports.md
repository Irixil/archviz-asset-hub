# Exporting

Damask has three ways to get your assets out: a one-off ZIP download of selected files, a CSV export of the activity log, and automated export configs that push a project's assets to an external destination on a schedule.

## Download selected assets as ZIP

Select one or more assets in the grid, create a `Stack`, then click **Download as ZIP** in the side panel.

![Export a stack as a ZIP file](/docs/screenshot_stack_zip.png)

## Automated exports

Automated export configs let Damask push all assets in a project to an external destination automatically, either on demand or whenever the project has been quiet for a set period.

Go to **Settings → Exports** to manage configs.

### Creating a config

![Create an automated export](/docs/screenshot_export_create1.png)
![Create an automated export](/docs/screenshot_export_create2.png)

| Field                | Description                                                                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Label**            | A name for your own reference                                                                                                                                            |
| **Project**          | The project whose assets are exported                                                                                                                                    |
| **Destination type** | `sftp` or `gdrive` (Google Drive)                                                                                                                                        |
| **Destination**      | Connection details for the chosen type (see below)                                                                                                                       |
| **Versions**         | `current`, only the latest version of each asset; `all`, every version                                                                                                   |
| **Include variants** | Whether to bundle variant files alongside the originals                                                                                                                  |
| **Schedule**         | `manual`, only runs when you click Trigger; `after quiet period`, runs automatically after no changes have been made to the project for the configured number of minutes |
| **Quiet period**     | Required when schedule is `after quiet period`. Minutes of inactivity before a run is triggered (1–10 080, i.e. up to one week)                                          |

### SFTP destination

| Field                   | Description                                                                        |
| ----------------------- | ---------------------------------------------------------------------------------- |
| **Host**                | Server hostname or IP                                                              |
| **Port**                | Usually `22`                                                                       |
| **Username**            | SSH username                                                                       |
| **Password**            | Password auth (leave blank if using a private key)                                 |
| **Private key**         | PEM-encoded private key (preferred over password auth)                             |
| **Remote path**         | Directory on the server where the export ZIP and manifest are written              |
| **Skip host-key check** | Disables SSH host-key verification. Not recommended outside of controlled networks |

### Google Drive destination

Connect your Google account first at **Settings → Integrations → Google Drive**, then:

| Field           | Description                                                |
| --------------- | ---------------------------------------------------------- |
| **Connection**  | The Google account connection to use                       |
| **Folder ID**   | ID of the Drive folder to write into (from the folder URL) |
| **Folder name** | Display name for your reference                            |

### Credential security

All passwords and private keys are encrypted at rest using AES-256-GCM. They are never returned in API responses after the initial save.

### Run history

Each time a config runs, a run record is created. Open a config to see its run history: status (`pending`, `running`, `done`, `failed`, or `partial`), assets exported, assets skipped, and bytes written. A `partial` status means some assets were exported and some were skipped (e.g. due to missing files).

### Enable / disable

Use the toggle on a config to pause it without deleting it. Disabled configs do not run automatically and cannot be triggered manually.
