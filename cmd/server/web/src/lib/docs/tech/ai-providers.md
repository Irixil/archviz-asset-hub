# AI providers

Damask integrates with two AI providers for image editing: **ImageRouter** and **OpenRouter**. Both support background removal and prompt-based image transformation.

## Supported providers

| Provider    | Background removal | Image editing (prompt) |
| ----------- | ------------------ | ---------------------- |
| ImageRouter | Yes                | Yes                    |
| OpenRouter  | Yes                | Yes                    |

You can configure one or both. If both are active, users can choose which provider and model to use per variant.

## Server-wide configuration

Set API keys and default models via environment variables. These apply to all workspaces unless overridden at the workspace level.

| Variable                              | Default                                    | Description                          |
| ------------------------------------- | ------------------------------------------ | ------------------------------------ |
| `IMAGEROUTER_API_KEY`                 | -                                          | ImageRouter API key                  |
| `IMAGEROUTER_DEFAULT_MODEL`           | `black-forest-labs/FLUX-2-klein-4b:free`   | Default model for image editing      |
| `IMAGEROUTER_DEFAULT_BG_REMOVE_MODEL` | `bria/remove-background:free`              | Default model for background removal |
| `OPENROUTER_API_KEY`                  | -                                          | OpenRouter API key                   |
| `OPENROUTER_DEFAULT_MODEL`            | `openai/dall-e-2`                          | Default model for image editing      |
| `OPENROUTER_DEFAULT_BG_REMOVE_MODEL`  | `stability-ai/stable-diffusion-xl-refiner` | Default model for background removal |

If neither key is set, AI editing will show as unavailable in the UI.

`APP_SECRET` must also be set , it is used to encrypt any workspace-level key overrides stored in the database.

## Workspace-level key override

Workspace owners can override the server-wide key (or set one when the env var is absent) per provider:

1. Go to **Settings → Integrations → AI services**
2. Expand the provider card (ImageRouter or OpenRouter)
3. Enter an API key and click **Save**
4. Use **Test** to verify the key is valid before saving

Clearing a workspace key reverts to the server-wide env var. If both are absent, the provider is shown as disabled.

## Key resolution order

For each request, Damask resolves the API key in this order:

1. **Workspace key** , set via the integrations settings page, encrypted at rest
2. **Environment variable** , server-wide fallback
3. **None** , provider appears in the model list as unconfigured; AI editing jobs using it will fail

## Model availability

Model lists are fetched live from each provider's API and cached for 5 minutes. If a provider's API is unreachable, the cached list is served. If no cache exists yet, an error is shown in the model picker.

## Next steps

- [Configuration](configuration) , full list of environment variables
- [AI editing](help/ai-editing) , how users create AI variants
