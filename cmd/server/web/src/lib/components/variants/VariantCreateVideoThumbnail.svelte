<script lang="ts">
  import { untrack } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { type Asset } from '$lib/api'
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

  const kind = 'video_capture_image'

  let videoTimestamp = $state(
    untrack(() => (initialParams?.timestamp as number) ?? 1)
  )
</script>

<div class="space-y-5">
  <div>
    <label for="variant-{kind}-timestamp" class="field-label"
      >{m.timestamp_seconds()}</label
    >
    <input
      id="variant-{kind}-timestamp"
      type="number"
      min="0"
      step="0.1"
      bind:value={videoTimestamp}
      class="field-input"
    />
  </div>
  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, { timestamp: videoTimestamp })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.extract_frame()}
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
</style>
