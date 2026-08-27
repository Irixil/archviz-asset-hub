<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { variantApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { isImage } from '$lib/utils/mime'
  import ResolutionOptions from '../ResolutionOptions.svelte'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    initialParams?: Record<string, unknown> | null
  }

  let { asset, creating, handleCreate, initialParams = null }: Props = $props()

  const kind = 'image_resize'

  let resizeWidth = $state(
    untrack(() => (initialParams?.width as number) ?? 800)
  )
  let resizeHeight = $state(
    untrack(() => (initialParams?.height as number) ?? 0)
  )
  let resizeFit = $state<'contain' | 'cover' | 'fill'>(
    untrack(
      () => (initialParams?.fit as 'contain' | 'cover' | 'fill') ?? 'contain'
    )
  )
  let resizeQuality = $state(
    untrack(() => (initialParams?.quality as number) ?? 85)
  )
  let resizeFormat = $state<'jpeg' | 'png' | 'tiff'>(
    untrack(() => (initialParams?.format as 'jpeg' | 'png' | 'tiff') ?? 'jpeg')
  )

  let previewUrl = $state('')
  let previewTimeout: ReturnType<typeof setTimeout>

  const isImageAsset = $derived(asset ? isImage(asset.mime_type) : false)

  function useResolution(e: Event) {
    const o = (e.target as HTMLSelectElement)?.selectedOptions
    if (!o) return
    const wh = o[0]?.getAttribute('data-wh')?.split('x')
    if (wh?.length === 2) {
      resizeWidth = +wh[0]
      resizeHeight = +wh[1]
    }
  }

  function updatePreview() {
    clearTimeout(previewTimeout)
    if (!asset || !isImageAsset) return
    previewTimeout = setTimeout(() => {
      if (!asset) return
      previewUrl = variantApi.previewUrl(asset.id, {
        w: resizeWidth || undefined,
        h: resizeHeight || undefined,
        fit: resizeFit,
        format: resizeFormat,
        q: resizeQuality,
      })
    }, 400)
  }
</script>

<div class="space-y-5">
  <div class="preview-area">
    {#if previewUrl}
      <img
        src={previewUrl}
        alt="Preview"
        class="max-h-48 max-w-full rounded object-contain"
      />
    {:else}
      <span class="preview-hint"
        >Preview will appear after changing parameters</span
      >
    {/if}
  </div>

  <div class="grid grid-cols-3 gap-3">
    <div>
      <label for="variant-{kind}-resolution" class="field-label"
        >{m.resolution()}</label
      >
      <select
        id="variant-{kind}-resolution"
        onchange={useResolution}
        class="field-input"
      >
        <ResolutionOptions />
      </select>
    </div>
    <div>
      <label for="variant-{kind}-width" class="field-label"
        >{m.width()} (px)</label
      >
      <input
        id="variant-{kind}-width"
        type="number"
        min="1"
        max="8000"
        bind:value={resizeWidth}
        oninput={updatePreview}
        class="field-input"
      />
    </div>
    <div>
      <label for="variant-{kind}-height" class="field-label"
        >{m.height()}
        <span class="text-[var(--text-muted)]">(0=auto)</span></label
      >
      <input
        id="variant-{kind}-height"
        type="number"
        min="0"
        max="8000"
        bind:value={resizeHeight}
        oninput={updatePreview}
        class="field-input"
      />
    </div>
  </div>

  <div>
    <label for="variant-{kind}-fit" class="field-label">Fit</label>
    <div class="flex gap-2">
      {#each ['contain', 'cover', 'fill'] as f}
        <button
          type="button"
          class="toggle-btn flex-1 {resizeFit === f ? 'active' : ''}"
          onclick={() => {
            resizeFit = f as typeof resizeFit
            updatePreview()
          }}>{f}</button
        >
      {/each}
    </div>
  </div>

  <div>
    <label for="variant-{kind}-quality" class="field-label"
      >{m.quality()}: <span class="quality-val">{resizeQuality}%</span></label
    >
    <input
      id="variant-{kind}-quality"
      type="range"
      min="1"
      max="100"
      bind:value={resizeQuality}
      oninput={updatePreview}
      class="range-input"
    />
  </div>

  <div>
    <label for="variant-{kind}-format" class="field-label">{m.format()}</label>
    <div class="flex gap-2">
      {#each ['jpeg', 'png', 'tiff'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {resizeFormat === fmt ? 'active' : ''}"
          onclick={() => {
            resizeFormat = fmt as typeof resizeFormat
            updatePreview()
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate?.(kind, {
        width: resizeWidth || undefined,
        height: resizeHeight || undefined,
        fit: resizeFit,
        quality: resizeQuality,
        format: resizeFormat,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_resize()}
  </Button>
</div>

<style>
  .preview-area {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 120px;
    border-radius: 10px;
    border: 1.5px dashed var(--border);
    background: var(--bg-app);
    padding: 12px;
  }
  .preview-hint {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
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
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
</style>
