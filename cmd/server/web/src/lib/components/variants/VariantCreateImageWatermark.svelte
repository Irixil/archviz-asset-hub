<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import {
    assetApi,
    variantApi,
    type Asset,
    type WatermarkAsset,
  } from '$lib/api'
  import { untrack } from 'svelte'
  import { generateDraft } from '$lib/api/drafts'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import VariantDraftSession from './VariantDraftSession.svelte'

  interface Props {
    asset: Asset
    onDone?: () => void
    onDraftStarted?: (nonce: string) => void
    sessionActive?: boolean
    initialParams?: Record<string, unknown> | null
  }

  let {
    asset,
    onDone,
    onDraftStarted,
    sessionActive = false,
    initialParams = null,
  }: Props = $props()

  type Phase = 'form' | 'drafting'

  const kind = 'image_watermark'
  let phase = $state<Phase>('form')
  let opacity = $state(
    untrack(() =>
      initialParams?.opacity != null
        ? (initialParams.opacity as number) * 100
        : 50
    )
  )
  let watermarkAsset = $state<WatermarkAsset | null>(null)
  let resolveError = $state('')
  let resolving = $state(false)
  let resolveToken = 0
  let submitting = $state(false)
  let sessionRef = $state<ReturnType<typeof VariantDraftSession> | undefined>(
    undefined
  )
  let watermarkTileWidth = $state<number | null>(null)
  let watermarkTileHeight = $state<number | null>(null)
  let previewFrameStyle = $derived(
    `aspect-ratio: ${asset.width && asset.height ? `${asset.width} / ${asset.height}` : '1 / 1'}; max-width: min(100%, ${220 * (asset.width && asset.height ? asset.width / asset.height : 1)}px);`
  )

  $effect(() => {
    const assetId = asset.id
    resolveToken += 1
    const token = resolveToken
    watermarkAsset = null
    resolveError = ''
    resolving = true

    void variantApi
      .resolveWatermark(assetId)
      .then((result) => {
        if (token !== resolveToken) return
        watermarkAsset = result
      })
      .catch((error: unknown) => {
        if (token !== resolveToken) return
        resolveError =
          error instanceof Error ? error.message : m.watermark_missing()
      })
      .finally(() => {
        if (token === resolveToken) resolving = false
      })
  })

  $effect(() => {
    if (!sessionActive) phase = 'form'
  })

  $effect(() => {
    const currentWatermark = watermarkAsset
    const assetWidth = asset.width
    const assetHeight = asset.height
    watermarkTileWidth = null
    watermarkTileHeight = null

    if (!currentWatermark || !assetWidth || !assetHeight) return
    if (typeof Image === 'undefined') return

    const image = new Image()
    const expectedSrc = assetApi.fileUrl(currentWatermark.id)
    image.src = expectedSrc
    image.onload = () => {
      if (image.src !== expectedSrc) return
      if (!image.naturalWidth || !image.naturalHeight) return
      watermarkTileWidth = (image.naturalWidth / assetWidth) * 100
      watermarkTileHeight = (image.naturalHeight / assetHeight) * 100
    }
  })

  async function handlePreview() {
    if (!watermarkAsset) return
    submitting = true
    try {
      const res = await generateDraft(asset.id, kind, {
        opacity: opacity / 100,
        watermark_asset_id: watermarkAsset.id,
      })
      if (onDraftStarted) {
        onDraftStarted(res.draft_key)
      } else {
        phase = 'drafting'
        await Promise.resolve()
        sessionRef?.addDraft(res.draft_key)
      }
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : m.variant_create_failed(),
        'error'
      )
    } finally {
      submitting = false
    }
  }

  function handleAddMore() {
    phase = 'form'
  }

  function handleDone() {
    onDone?.()
  }
</script>

{#if phase === 'form' || onDraftStarted}
  <div class="space-y-5">
    <div class="preview-card">
      {#if watermarkAsset}
        <div class="preview-head">
          <div>
            <div class="preview-title">{watermarkAsset.name}</div>
            <div class="preview-scope">
              {m.watermark_scope_label()}: {watermarkAsset.scope}
            </div>
          </div>
          <span class="scope-pill">{watermarkAsset.scope}</span>
        </div>
        <div class="preview-stage">
          <div class="preview-frame" style={previewFrameStyle}>
            <img
              class="preview-img"
              src={assetApi.thumbUrl(asset.id, asset.thumbnail_key)}
              alt={asset.original_filename}
            />
            <div
              aria-hidden="true"
              class="watermark-overlay"
              style={`opacity: ${opacity / 100}; background-image: url('${assetApi.fileUrl(watermarkAsset.id)}'); ${watermarkTileWidth && watermarkTileHeight ? `background-size: ${watermarkTileWidth}% ${watermarkTileHeight}%;` : ''}`}
            ></div>
          </div>
        </div>
      {:else if resolving}
        <div class="preview-empty">{m.loading()}</div>
      {:else}
        <div class="preview-empty error-text">
          {resolveError || m.watermark_missing()}
        </div>
      {/if}
    </div>

    <div>
      <label for="variant-{kind}-opacity" class="field-label">
        {m.opacity()}: <span class="quality-val">{opacity}%</span>
      </label>
      <input
        id="variant-{kind}-opacity"
        type="range"
        min="1"
        max="100"
        bind:value={opacity}
        class="range-input"
      />
    </div>

    <Button
      disabled={submitting || authStore.role === 'viewer' || !watermarkAsset}
      onclick={handlePreview}
      class="w-full"
    >
      {submitting ? m.queuing_() : m.variants_draft_preview_button()}
    </Button>
  </div>
{:else}
  <VariantDraftSession
    bind:this={sessionRef}
    assetId={asset.id}
    onDone={handleDone}
    onAddMore={handleAddMore}
    onRestoreSession={() => {
      phase = 'drafting'
    }}
  />
{/if}

<style>
  .preview-card {
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 12px;
    background: var(--bg-surface);
  }
  .preview-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
  }
  .preview-title {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .preview-scope {
    font-size: 0.75rem;
    color: var(--text-secondary);
    text-transform: capitalize;
  }
  .scope-pill {
    border-radius: 999px;
    padding: 4px 8px;
    background: var(--bg-subtle);
    color: var(--text-secondary);
    font-size: 0.7rem;
    text-transform: capitalize;
  }
  .preview-stage {
    display: flex;
    justify-content: center;
    padding: 8px;
    background: var(--bg-base);
    border-radius: 8px;
  }
  .preview-frame {
    position: relative;
    display: block;
    width: 100%;
    max-width: 100%;
    max-height: 220px;
    overflow: hidden;
    border-radius: 8px;
    background: #000;
  }
  .preview-img {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .watermark-overlay {
    position: absolute;
    inset: 0;
    border-radius: 8px;
    background-position: top left;
    background-repeat: repeat;
    pointer-events: none;
  }
  .preview-empty {
    display: grid;
    place-items: center;
    min-height: 120px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }
  .error-text {
    color: var(--danger, #b42318);
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
</style>
