# Sharing

Share assets with clients or collaborators without requiring them to have a Damask account.

## Create a share link

Share links can be created from two places:

- **From the sidebar**, hover over a project name and click the share icon, or open the project detail page and click **Share project**
- **From selected assets**, select one or more assets in the grid, then click **Share** in the top bar or press `Ctrl+Shift+S`

Both open the same share creation panel.

Configure the share:

| Setting            | Description                                                      |
| ------------------ | ---------------------------------------------------------------- |
| **Name**           | A label for your own reference (e.g. "Nike Q3, Client Delivery") |
| **Password**       | Optional. Visitors must enter this before viewing                |
| **Expiry**         | Optional. The link stops working after this date                 |
| **Allow comments** | Whether visitors can leave comments on individual assets         |
| **Allow download** | Whether visitors can download the original files                 |

After saving, click the copy button to grab the generated link.

![New share creation panel](/docs/screenshot_share_new.png)

## Share page

Visitors open the link in any browser. If password-protected, they enter the password once, a session is valid for 24 hours on that device. If the link has expired, they see a clear message rather than a broken page.

The gallery shows a grid of asset thumbnails. Clicking an asset opens a review panel with the full preview, a download button (if enabled), and the comment thread for that asset.

![Shared gallery view](/docs/screenshot_shared_gallery.png)

## Comments

When **Allow comments** is enabled, visitors can leave feedback directly on individual assets inside the gallery. They provide a name and optional email, no account required.

![Visitor leaving a comment on a shared asset](/docs/screenshot_shared_asset_comment.png)

Comments appear on the share page in real time. As the share owner, you can see all comments:

1. **In the asset detail panel**, the Activity tab shows all comments received via share links
2. **In Settings → Shares**, click any share, then open the **Comments** tab to see all comments grouped by asset and delete any of them

## Managing shares

Go to **Settings → Shares** to see all active and revoked share links. Each entry shows the label, target, status, view count, and comment count. From here you can edit settings, copy the link, or revoke it.

Revoking a share link stops it from working immediately. The share record is retained in history so that clients who open an old link see a clear "This link has been revoked" message rather than a 404 error.

## View tracking

Every time someone successfully accesses a share link (past the password gate), the view count increments. Individual download events are also logged in the asset's activity log.

## Exporting

When **Allow download** is enabled on a share, visitors (and you) can download a ZIP of all assets in the share from the share page footer.

For bulk downloads of assets you own (outside of a share link), or for automated scheduled exports to SFTP or Google Drive, see [Exporting](help/exports).
