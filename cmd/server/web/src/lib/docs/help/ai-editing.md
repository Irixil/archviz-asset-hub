# AI editing

AI editing lets you transform images using generative AI models , remove backgrounds, apply style changes, or edit content with a text prompt.

## Prerequisites

A workspace owner must configure at least one AI provider before AI editing is available. Go to **Settings → Integrations → AI services** and add an API key for ImageRouter, OpenRouter, or both.

If no provider is configured, the AI editing option will appear as unavailable in the variant panel.

## Creating an AI variant

1. Open an image asset
2. Go to the **Variants** tab
3. Click **+ New variant** and select **AI editing**
4. Choose a capability: **Background removal** or **Image editing** (prompt-based)
5. Select a provider and model
6. Enter a prompt if using image editing
7. Click **Generate previews**

![AI variant panel](/docs/screenshot_asset_variant_ai.png)

## Provider and model selection

The model picker shows all available models from configured providers. Each model displays:

- The provider it belongs to (ImageRouter or OpenRouter)
- The price per image (or "free" for zero-cost models)
- Which capabilities it supports

Use the **multi-model toggle** (layers icon) to select several models at once. Damask generates one preview per model, letting you compare results side by side before committing.

![AI model selector](/docs/screenshot_asset_variant_ai_models.png)

## Capabilities

| Capability             | What it does                                          |
| ---------------------- | ----------------------------------------------------- |
| **Background removal** | Removes the background and produces a transparent PNG |
| **Image editing**      | Transforms the image according to a text prompt       |

Not all models support both capabilities. The model picker only shows models that match the selected capability.

## Preview workflow

After clicking **Generate previews**, Damask sends the image to each selected model and displays the results as preview cards.

![AI variant previews](/docs/screenshot_asset_variant_ai_previews.png)

From the preview panel you can:

- **Keep** a result , saves it as a named variant attached to the asset
- **Discard** a result , removes the preview without saving
- **Keep all** or **Discard all** , bulk actions for when you've generated multiple previews

Previews are temporary. Navigating away without keeping a result discards it automatically.

## Free vs. paid models

Models marked **free** incur no cost against your API key. Paid models deduct from your provider account balance at the rate shown per image. Always check pricing in the model picker before running large batches.

## Next steps

- [Variants](variants) , promoting a result, re-running variants, manual upload
- [Workflows](workflows) , automating AI edits on asset upload
