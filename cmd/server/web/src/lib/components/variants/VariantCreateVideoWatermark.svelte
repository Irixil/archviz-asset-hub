<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import ResolutionOptions from '../ResolutionOptions.svelte'
  import {
    assetApi,
    variantApi,
    type Asset,
    type WatermarkAsset,
  } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    initialParams?: Record<string, unknown> | null
  }

  let { asset, creating, handleCreate, initialParams = null }: Props = $props()

  const kind = 'video_watermark'
  let opacity = $state(
    untrack(() =>
      initialParams?.opacity != null
        ? (initialParams.opacity as number) * 100
        : 50
    )
  )
  let videoFormat = $state<'mp4' | 'webm'>(
    untrack(() => (initialParams?.format as 'mp4' | 'webm') ?? 'mp4')
  )
  let videoResolution = $state<'' | '1080p' | '720p' | '480p'>(
    untrack(
      () => (initialParams?.resolution as '' | '1080p' | '720p' | '480p') ?? ''
    )
  )
  let stripAudio = $state(
    untrack(() => (initialParams?.strip_audio as boolean) ?? false)
  )
  let watermarkAsset = $state<WatermarkAsset | null>(null)
  let resolveError = $state('')
  let resolving = $state(false)
  let resolveToken = 0
  let watermarkTileWidth = $state<number | null>(null)
  let watermarkTileHeight = $state<number | null>(null)
  let previewAspect = $derived(
    asset.width && asset.height ? asset.width / asset.height : 16 / 9
  )
  let previewFrameStyle = $derived(
    `aspect-ratio: ${asset.width && asset.height ? `${asset.width} / ${asset.height}` : '16 / 9'}; max-width: min(100%, ${220 * previewAspect}px);`
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
    const currentWatermark = watermarkAsset
    const assetWidth = asset.width
    const assetHeight = asset.height
    watermarkTileWidth = null
    watermarkTileHeight = null

    if (!currentWatermark || !assetWidth || !assetHeight) return
    if (typeof Image === 'undefined') return

    const image = new Image()
    image.src = assetApi.fileUrl(currentWatermark.id)
    image.onload = () => {
      if (currentWatermark !== watermarkAsset) return
      if (!image.naturalWidth || !image.naturalHeight) return

      watermarkTileWidth = (image.naturalWidth / assetWidth) * 100
      watermarkTileHeight = (image.naturalHeight / assetHeight) * 100
    }
  })
</script>

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
          <video
            class="preview-video"
            src={assetApi.fileUrl(asset.id)}
            muted
            autoplay
            loop
            playsinline
            controls={false}
          ></video>
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
    <label for="variant-{kind}-opacity" class="field-label"
      >{m.opacity()}:
      <span class="quality-val">{opacity}%</span></label
    >
    <input
      id="variant-{kind}-opacity"
      type="range"
      min="1"
      max="100"
      bind:value={opacity}
      class="range-input"
    />
  </div>

  <div>
    <label for="variant-{kind}-format" class="field-label"
      >{m.output_format()}</label
    >
    <div class="flex gap-2">
      {#each ['mp4', 'webm'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {videoFormat === fmt ? 'active' : ''}"
          onclick={() => {
            videoFormat = fmt as typeof videoFormat
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  <div>
    <label for="variant-{kind}-resolution" class="field-label"
      >{m.resolution()} <span class="optional">({m.optional()})</span></label
    >
    <select
      id="variant-{kind}-resolution"
      bind:value={videoResolution}
      class="field-input"
    >
      <ResolutionOptions />
    </select>
  </div>

  <label class="checkbox-label">
    <input type="checkbox" bind:checked={stripAudio} class="checkbox" />
    {m.strip_audio()}
  </label>

  <Button
    disabled={creating || authStore.role === 'viewer' || !watermarkAsset}
    onclick={() =>
      handleCreate(kind, {
        opacity: opacity / 100,
        format: videoFormat,
        resolution: videoResolution || undefined,
        strip_audio: stripAudio,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_watermark()}
  </Button>
</div>

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
  .preview-video {
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
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .optional {
    font-weight: 400;
    color: var(--text-muted);
  }
  .field-input {
    width: 100%;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-primary);
    padding: 7px 10px;
    font-size: 0.875rem;
    outline: none;
    transition: border-color 0.12s ease;
  }
  .field-input:focus {
    border-color: var(--accent-cta);
  }
  .toggle-btn {
    border-radius: 7px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-secondary);
    padding: 7px 0;
    font-size: 0.8125rem;
    font-weight: 500;
    transition: all 0.1s ease;
    cursor: pointer;
  }
  .toggle-btn:hover {
    border-color: var(--accent-cta);
    color: var(--text-primary);
  }
  .toggle-btn.active {
    border-color: var(--accent-cta);
    background: oklch(93% 0.04 270);
    color: oklch(40% 0.18 270);
  }
  :global(.dark) .toggle-btn.active {
    background: oklch(30% 0.08 270 / 0.4);
    color: oklch(78% 0.12 270);
  }
  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 0.875rem;
    color: var(--text-primary);
    cursor: pointer;
  }
  .checkbox {
    border-radius: 4px;
    accent-color: var(--accent-cta);
  }
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
  .error-text {
    color: var(--danger, #b42318);
  }
</style>
