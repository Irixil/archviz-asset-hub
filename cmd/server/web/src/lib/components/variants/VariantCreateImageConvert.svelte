<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'
  import { type Asset } from '$lib/api'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
    initialParams?: Record<string, unknown> | null
  }

  let {
    asset: _asset,
    creating,
    handleCreate,
    initialParams = null,
  }: Props = $props()

  const kind = 'image_convert'

  let convertFormat = $state<'jpeg' | 'png' | 'tiff' | 'webp'>(
    untrack(
      () => (initialParams?.format as 'jpeg' | 'png' | 'tiff' | 'webp') ?? 'png'
    )
  )
  let convertQuality = $state(
    untrack(() => (initialParams?.quality as number) ?? 90)
  )
</script>

<div class="space-y-5">
  <div>
    <label for="variant-{kind}-format" class="field-label"
      >{m.output_format()}</label
    >
    <div class="flex gap-2">
      {#each ['jpeg', 'png', 'webp', 'tiff'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {convertFormat === fmt ? 'active' : ''}"
          onclick={() => {
            convertFormat = fmt as typeof convertFormat
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  {#if convertFormat === 'jpeg'}
    <div>
      <label for="variant-{kind}-quality" class="field-label"
        >{m.quality()}:
        <span class="quality-val">{convertQuality}%</span></label
      >
      <input
        id="variant-{kind}-quality"
        type="range"
        min="1"
        max="100"
        bind:value={convertQuality}
        class="range-input"
      />
    </div>
  {:else if convertFormat === 'webp'}
    <p class="format-note">WebP export is lossless. Quality does not apply.</p>
  {/if}

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate(kind, { format: convertFormat, quality: convertQuality })}
    class="w-full"
  >
    {creating
      ? m.queuing_()
      : m.convert_to_format({ format: convertFormat.toUpperCase() })}
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
  .toggle-btn {
    border-radius: 7px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-secondary);
    padding: 9px 0;
    font-size: 0.875rem;
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
  .format-note {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
</style>
