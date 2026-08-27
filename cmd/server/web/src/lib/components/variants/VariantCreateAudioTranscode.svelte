<script lang="ts">
  import { fly } from 'svelte/transition'
  import { untrack } from 'svelte'
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import VariantFormatSelect from './VariantFormatSelect.svelte'
  import BitrateSelect from './BitrateSelect.svelte'
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

  const kind = 'audio_transcode'
  const formats = [
    { value: 'mp3', label: 'MP3' },
    { value: 'aac', label: 'AAC' },
    { value: 'opus', label: 'Opus' },
    { value: 'ogg', label: 'OGG' },
    { value: 'flac', label: 'FLAC' },
    { value: 'wav', label: 'WAV' },
  ]

  let format = $state(untrack(() => (initialParams?.format as string) ?? 'mp3'))
  let bitrate = $state(
    untrack(() => (initialParams?.bitrate as string) ?? '192k')
  )
  let mono = $state(untrack(() => (initialParams?.mono as boolean) ?? false))
  const lossless = $derived(format === 'flac' || format === 'wav')
</script>

<div class="space-y-5">
  <div class="form-header">
    <p class="form-desc">{m.audio_transcode_description()}</p>
  </div>

  <VariantFormatSelect
    id="variant-{kind}-format"
    value={format}
    options={formats}
    label={m.audio_format_label()}
    onchange={(next) => (format = next)}
  />

  {#if lossless}
    <p transition:fly={{ y: -6, duration: 140 }} class="explainer">
      {m.audio_lossless_note()}
    </p>
  {:else}
    <div transition:fly={{ y: -6, duration: 140 }}>
      <BitrateSelect
        id="variant-{kind}-bitrate"
        value={bitrate}
        label={m.audio_bitrate_label()}
        onchange={(next) => (bitrate = next)}
      />
    </div>
  {/if}

  <label class="checkbox-label">
    <input type="checkbox" bind:checked={mono} class="checkbox" />
    {m.audio_mono_label()}
  </label>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate(kind, {
        format,
        bitrate: lossless ? undefined : bitrate,
        mono,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.audio_transcode_submit()}
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
  .explainer {
    border-radius: 8px;
    background: var(--bg-app);
    padding: 10px 12px;
    font-size: 0.8125rem;
    color: var(--text-muted);
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
