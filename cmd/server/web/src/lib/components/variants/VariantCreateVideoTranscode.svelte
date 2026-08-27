<script lang="ts">
  import { untrack } from 'svelte'
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import ResolutionOptions from '../ResolutionOptions.svelte'
  import { m } from '$lib/paraglide/messages'

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

  const kind = 'video_transcode'

  let transcodeFormat = $state<'mp4' | 'webm'>(
    untrack(() => (initialParams?.format as 'mp4' | 'webm') ?? 'mp4')
  )
  let transcodeResolution = $state<'' | '1080p' | '720p' | '480p'>(
    untrack(
      () => (initialParams?.resolution as '' | '1080p' | '720p' | '480p') ?? ''
    )
  )
  let transcodeStripAudio = $state(
    untrack(() => (initialParams?.strip_audio as boolean) ?? false)
  )
</script>

<div class="space-y-5">
  <div class="form-header">
    <p class="form-desc">{m.transcode_hint()}</p>
  </div>

  <div>
    <label for="variant-{kind}-format" class="field-label"
      >{m.output_format()}</label
    >
    <div class="flex gap-2">
      {#each ['mp4', 'webm'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {transcodeFormat === fmt ? 'active' : ''}"
          onclick={() => {
            transcodeFormat = fmt as typeof transcodeFormat
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
      bind:value={transcodeResolution}
      class="field-input"
    >
      <ResolutionOptions />
    </select>
  </div>

  <label class="checkbox-label">
    <input
      type="checkbox"
      bind:checked={transcodeStripAudio}
      class="checkbox"
    />
    {m.strip_audio()}
  </label>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate(kind, {
        format: transcodeFormat,
        resolution: transcodeResolution || undefined,
        strip_audio: transcodeStripAudio,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.transcode()}
  </Button>
</div>

<style>
  .form-header {
    padding-bottom: 4px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .form-desc {
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
</style>
