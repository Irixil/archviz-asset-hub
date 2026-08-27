<script lang="ts">
  import { untrack } from 'svelte'
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import VariantFormatSelect from './VariantFormatSelect.svelte'
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

  const kind = 'audio_normalize'
  const formats = [
    { value: 'source', label: m.audio_format_source() },
    { value: 'mp3', label: 'MP3' },
    { value: 'aac', label: 'AAC' },
    { value: 'wav', label: 'WAV' },
  ]
  const targets = [
    { value: -14, label: m.audio_target_streaming() },
    { value: -16, label: m.audio_target_general() },
    { value: -23, label: m.audio_target_broadcast() },
  ]

  let targetLUFS = $state(
    untrack(() => (initialParams?.target_lufs as number) ?? -16)
  )
  let format = $state(
    untrack(() => (initialParams?.format as string) ?? 'source')
  )
</script>

<div class="space-y-5">
  <div class="form-header">
    <p class="form-desc">{m.audio_normalize_description()}</p>
  </div>

  <div>
    <label for="variant-{kind}-target" class="field-label"
      >{m.audio_target_label()}</label
    >
    <select
      id="variant-{kind}-target"
      value={targetLUFS}
      class="field-input"
      onchange={(e) =>
        (targetLUFS = Number((e.currentTarget as HTMLSelectElement).value))}
    >
      {#each targets as target}
        <option value={target.value}>{target.label}</option>
      {/each}
    </select>
  </div>

  <VariantFormatSelect
    id="variant-{kind}-format"
    value={format}
    options={formats}
    label={m.audio_output_format_label()}
    onchange={(next) => (format = next)}
  />

  <p class="explainer">{m.audio_normalize_explainer()}</p>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, { target_lufs: targetLUFS, format })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.audio_normalize_submit()}
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
  .explainer {
    border-radius: 8px;
    background: var(--bg-app);
    padding: 10px 12px;
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
</style>
