<script lang="ts">
  import { variantApi, type Variant } from '$lib/api'
  import { formatBytes } from '$lib/api'
  import { toastStore } from '$lib/stores/toast.svelte'

  interface Props {
    assetId: string
    assetFilename: string
    variant: Variant
    onCancel: () => void
    onSuccess: (newAssetId: string) => void
  }

  let { assetId, assetFilename, variant, onCancel, onSuccess }: Props = $props()

  function stemOf(filename: string) {
    const dot = filename.lastIndexOf('.')
    return dot > 0 ? filename.slice(0, dot) : filename
  }

  let name = $state('')
  let error = $state('')
  let submitting = $state(false)

  $effect(() => {
    name =
      `${stemOf(assetFilename)} ${variant.type.replaceAll('_', ' ')}`.trim()
  })

  async function submit() {
    submitting = true
    error = ''
    try {
      const result = await variantApi.promote(assetId, variant.id, name)
      toastStore.show('New asset created', 'success')
      onSuccess(result.asset_id)
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : 'Failed to create asset.'
    } finally {
      submitting = false
    }
  }
</script>

<div class="promote-panel">
  <div class="promote-head">
    <button type="button" class="back-button" onclick={onCancel}>←</button>
    <h3>Create new asset from variant</h3>
  </div>

  <div class="variant-summary">
    <div class="variant-thumb">
      {#if variant.thumbnail_url}
        <img src={variant.thumbnail_url} alt={variant.type} />
      {:else}
        <div class="thumb-placeholder">No preview</div>
      {/if}
    </div>
    <div>
      <p class="variant-title">{variant.type}</p>
      <p class="variant-meta">
        {variant.size ? formatBytes(variant.size) : 'Unknown size'}
      </p>
    </div>
  </div>

  <label class="field">
    <span>Name</span>
    <input bind:value={name} maxlength={255} disabled={submitting} />
  </label>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <p class="hint">
    Tags and custom fields will be copied from this asset. The variant will be
    deleted after the asset is created.
  </p>

  <div class="actions">
    <button
      type="button"
      class="secondary"
      onclick={onCancel}
      disabled={submitting}>Cancel</button
    >
    <button
      type="button"
      class="primary"
      onclick={submit}
      disabled={submitting || !name.trim()}
    >
      {submitting ? 'Creating…' : 'Create asset →'}
    </button>
  </div>
</div>

<style>
  .promote-panel {
    display: grid;
    gap: 16px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: var(--bg-surface);
    padding: 18px;
  }
  .promote-head {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .promote-head h3 {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text-primary);
  }
  .back-button {
    border: 0;
    background: transparent;
    color: var(--text-secondary);
    font-size: 1.1rem;
    cursor: pointer;
  }
  .variant-summary {
    display: flex;
    gap: 14px;
    align-items: center;
  }
  .variant-thumb {
    width: 80px;
    height: 80px;
    border-radius: 12px;
    overflow: hidden;
    background: var(--bg-app);
    border: 1px solid var(--border);
  }
  .variant-thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .thumb-placeholder {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
    color: var(--text-muted);
    font-size: 0.75rem;
  }
  .variant-title {
    font-weight: 600;
    color: var(--text-primary);
  }
  .variant-meta,
  .hint {
    color: var(--text-muted);
    font-size: 0.875rem;
  }
  .field {
    display: grid;
    gap: 6px;
  }
  .field span {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-secondary);
  }
  .field input {
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 10px 12px;
    background: var(--bg-app);
    color: var(--text-primary);
  }
  .error {
    color: var(--accent-danger);
    font-size: 0.875rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }
  .actions button {
    border-radius: 10px;
    padding: 10px 14px;
    font-weight: 600;
    cursor: pointer;
  }
  .secondary {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
  }
  .primary {
    border: 0;
    background: var(--accent-cta);
    color: white;
  }
</style>
