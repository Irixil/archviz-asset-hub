<script lang="ts">
  import { variantApi, type Variant } from '$lib/api'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import { m } from '$lib/paraglide/messages'
  import { toastStore } from '$lib/stores/toast.svelte'

  interface Props {
    variants: readonly Variant[]
    assetId: string
    onUpdate: (updated: readonly Variant[]) => void
  }

  let { variants, assetId, onUpdate }: Props = $props()
  let loading = $state(false)

  const sharedCount = $derived(
    variants.filter((variant) => variant.is_shared).length
  )
  const allShared = $derived(
    variants.length > 0 && sharedCount === variants.length
  )

  async function setAll(nextValue: boolean) {
    loading = true
    try {
      const updates = Object.fromEntries(
        variants.map((variant) => [variant.id, nextValue])
      )
      const res = await variantApi.updateSharing(assetId, updates)
      onUpdate(res.variants)
    } catch (error) {
      toastStore.show(
        error instanceof Error ? error.message : 'Failed to update sharing.',
        'error'
      )
    } finally {
      loading = false
    }
  }
</script>

{#if variants.length > 0}
  <div class="sharing-header">
    <div>
      <p class="sharing-kicker">{m.variants_visibility()}</p>
      <p class="sharing-copy">{sharedCount} of {variants.length} shared</p>
    </div>
    <div class="sharing-actions">
      <button
        type="button"
        class:active={allShared}
        disabled={loading || variants.length === 0}
        onclick={() => setAll(true)}
      >
        {m.share_all()}
      </button>
      <button
        type="button"
        disabled={loading || variants.length === 0}
        onclick={() => setAll(false)}
      >
        {m.make_all_private()}
      </button>
      {#if loading}
        <Spinner size="sm" />
      {/if}
    </div>
  </div>
{/if}

<style>
  .sharing-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
    padding: 12px 16px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-surface);
  }
  .sharing-kicker {
    margin: 0;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .sharing-copy {
    margin: 3px 0 0;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
  }
  .sharing-actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .sharing-actions button {
    padding: 6px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-app);
    color: var(--text-secondary);
    font-size: 0.8125rem;
    font-weight: 500;
    transition:
      background 0.12s ease,
      border-color 0.12s ease,
      color 0.12s ease;
  }
  .sharing-actions button:hover:not(:disabled) {
    background: var(--bg-hover);
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .sharing-actions button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .sharing-actions button.active {
    border-color: var(--accent-cta);
    background: color-mix(in srgb, var(--accent-cta) 8%, transparent);
    color: var(--accent-cta);
  }
</style>
