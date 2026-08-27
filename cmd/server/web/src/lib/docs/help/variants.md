# Variants

Variants are derived versions of an asset, different sizes, formats, or processed copies.

## What are variants?

A variant is a processed copy of an original asset, stored separately. Common uses:

- Resized images for web delivery
- Format conversions (JPEG → WebP, MP4 → WebM)
- Cropped regions for specific placements
- Watermarked proofs for client review
- Background-removed PNG cutouts
- Video thumbnail extraction

Variants never replace the original. You can always download the source file regardless of how many variants exist.

## Creating variants

1. Open an asset
2. Go to the **Variants** tab
3. Click **+ New variant** and choose a type
4. Configure the parameters, a live preview updates as you adjust settings
5. Click **Create**, Damask enqueues a background job and the variant appears when processing completes

![Variants tab on an asset](/docs/screenshot_asset_variants.png)

## Variant types

| Type                   | What it does                                                                                                       |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------ |
| **Resize**             | Scale to a specific width/height with fit options (cover, contain, fill) and output format (JPEG, PNG, WebP, AVIF) |
| **Convert**            | Change format without resizing, images to JPEG/PNG/WebP/AVIF, video to MP4/WebM                                    |
| **Crop**               | Export a selected region, optionally locked to an aspect ratio                                                     |
| **Background removal** | Remove the image background, producing a transparent PNG                                                           |
| **Video thumbnail**    | Extract a frame from a video as an image                                                                           |
| **AI editing**         | Create new images from an asset, given an LLM prompt and one or more models                                        |
| **Workflow**           | Trigger a manual workflow                                                                                          |

## About AI variants

Some variants allow you to preview results before committing the newly created variant.
The AI variant is one of them. Use the AI variant tool to edit as an asset, and preview the result. This tool allow multiple models selection so you can try different generative models at once.

![AI variant preview](/docs/screenshot_asset_variant_ai_previews.png)

Generated previews can then be discarded or kept, one by one or all at once.

## Automating variants with workflows

Variants can be created automatically using **Workflows**. A workflow triggered on asset upload (or any other event) can include a **Create Variant** action that queues a variant job for every matching asset, for example, generating a WebP resize of every uploaded image in a given project.

See [Workflows](workflows) for a full reference.

## Promoting a variant

If a variant should become the primary file (e.g. a retouched version or a format that better suits your workflow), click **Promote** on the variant. This swaps it in as the asset's primary file. The original becomes a variant in its place.

## Manual upload

You can also upload a manually processed file as a variant using **Upload variant** on the asset detail page, useful when you've processed the file in an external tool.

## Rerunning a variant

If the source asset has been updated (new version uploaded), click **Re-run** on any variant to regenerate it from the latest file.
