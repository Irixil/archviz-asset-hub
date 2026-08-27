<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import ResolutionOptions from '../ResolutionOptions.svelte'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    initialParams?: Record<string, unknown> | null
  }

  let { asset, creating, handleCreate, initialParams = null }: Props = $props()

  const kind = 'image_smart_crop'

  let width = $state(untrack(() => (initialParams?.width as number) ?? 400))
  let height = $state(untrack(() => (initialParams?.height as number) ?? 400))
  let format = $state<'jpeg' | 'png' | 'tiff'>(
    untrack(() => (initialParams?.format as 'jpeg' | 'png' | 'tiff') ?? 'jpeg')
  )
  let quality = $state(untrack(() => (initialParams?.quality as number) ?? 85))

  function useResolution(e: Event) {
    const o = (e.target as HTMLSelectElement)?.selectedOptions
    if (!o) return
    const wh = o[0]?.getAttribute('data-wh')?.split('x')
    if (wh?.length === 2) {
      width = +wh[0]
      height = +wh[1]
    }
  }
</script>

<div class="space-y-5">
  {#if asset.width != null && asset.height != null}
    <p class="text-sm text-[var(--text-muted)]">
      Original: {asset.width} × {asset.height} px
    </p>
  {/if}

  <div class="grid grid-cols-3 gap-4">
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
        bind:value={width}
        class="field-input"
      />
    </div>
    <div>
      <label for="variant-{kind}-height" class="field-label"
        >{m.height()} (px)</label
      >
      <input
        id="variant-{kind}-height"
        type="number"
        min="1"
        max="8000"
        bind:value={height}
        class="field-input"
      />
    </div>
  </div>

  <div>
    <label for="variant-{kind}-format" class="field-label">{m.format()}</label>
    <div class="flex gap-2">
      {#each ['jpeg', 'png', 'tiff'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {format === fmt ? 'active' : ''}"
          onclick={() => {
            format = fmt as typeof format
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  <div>
    <label for="variant-{kind}-quality" class="field-label"
      >{m.quality()}: <span class="quality-val">{quality}%</span></label
    >
    <input
      id="variant-{kind}-quality"
      type="range"
      min="1"
      max="100"
      bind:value={quality}
      class="range-input"
    />
  </div>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, { width, height, format, quality })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_smartcrop()}
  </Button>
</div>

<style>
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
