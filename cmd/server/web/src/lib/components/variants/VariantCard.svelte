<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import {
    Check,
    Download,
    ImageOff,
    MoreHorizontal,
    Pencil,
    X,
  } from '@lucide/svelte'
  import {
    apiFetchRaw,
    formatBytes,
    mimeCategory,
    variantApi,
    type Variant,
  } from '$lib/api'
  import AssetThumbnail from '$lib/components/asset/AssetThumbnail.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'

  interface Props {
    variant: Variant
    assetId: string
    assetMimeType: string
    isSelected: boolean
    onSelect: () => void
    onVariantUpdated: (variant: Variant) => void
    onVariantsUpdated: (variants: readonly Variant[]) => void
    onDelete: () => void
    onPromote: () => void
    onThumbnailUpdated: () => void
    onRerun: () => void
    onReuse: (variant: Variant) => void
  }

  let {
    variant,
    assetId,
    assetMimeType,
    isSelected,
    onSelect,
    onVariantUpdated,
    onVariantsUpdated,
    onDelete,
    onPromote,
    onThumbnailUpdated,
    onRerun,
    onReuse,
  }: Props = $props()

  const category = $derived(mimeCategory(assetMimeType))
  const thumbBg = $derived(ASSET_BACKGROUND_COLORS[category] ?? '')

  let thumbUrl = $state<string | null>(null)
  let thumbContentType = $state('image/jpeg')
  let noPreview = $state(false)
  let menuOpen = $state(false)
  let editingTitle = $state(false)
  let draftTitle = $state('')
  let titleBusy = $state(false)
  let shareBusy = $state(false)
  let showDeleteConfirm = $state(false)
  let ignoreTitleBlur = $state(false)

  const MAX_POLL = 30
  let pollCount = 0
  let pollTimer: ReturnType<typeof setTimeout> | null = null

  $effect(() => {
    thumbUrl = variant.thumbnail_url ?? null
    thumbContentType = variant.thumbnail_content_type || 'image/jpeg'
    noPreview = false
    draftTitle = variant.title
    pollCount = 0
    if (pollTimer) clearTimeout(pollTimer)
    startPolling()
  })

  function startPolling() {
    if (thumbUrl || noPreview) return
    poll()
  }

  async function poll() {
    if (pollCount >= MAX_POLL) {
      noPreview = true
      return
    }
    pollCount++
    try {
      const res = await apiFetchRaw(
        `/api/v1/assets/${assetId}/variants/${variant.id}/thumb`,
        { headers: {} }
      )
      if (res.status === 202) {
        pollTimer = setTimeout(poll, 2000)
      } else if (res.ok) {
        thumbUrl = variantApi.thumbUrl(assetId, variant.id)
        const ct = res.headers.get('Content-Type')
        if (ct) thumbContentType = ct.split(';')[0].trim()
      } else {
        noPreview = true
      }
    } catch {
      noPreview = true
    }
  }

  onMount(() => startPolling())
  onDestroy(() => {
    if (pollTimer) clearTimeout(pollTimer)
  })

  async function setAsThumbnail() {
    try {
      await variantApi.setThumbnail(assetId, variant.id)
      toastStore.show('缩略图已更新', 'success')
      menuOpen = false
      onThumbnailUpdated()
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : '缩略图更新失败，请重试',
        'error'
      )
    }
  }

  async function rerunVariant() {
    let params: Record<string, unknown> | undefined
    if (variant.transform_params) {
      try {
        params = JSON.parse(variant.transform_params) as Record<string, unknown>
      } catch {
        params = undefined
      }
    }
    try {
      await variantApi.rerun(assetId, variant.id, params)
      toastStore.show('Variant queued for re-run', 'success')
      menuOpen = false
      onRerun()
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to re-run variant.',
        'error'
      )
    }
  }

  async function saveTitle() {
    titleBusy = true
    try {
      const updated = await variantApi.patch(assetId, variant.id, {
        title: draftTitle,
      })
      onVariantUpdated(updated)
      editingTitle = false
    } catch (error) {
      toastStore.show(
        error instanceof Error ? error.message : 'Failed to update title.',
        'error'
      )
    } finally {
      titleBusy = false
    }
  }

  function cancelEdit() {
    draftTitle = variant.title
    editingTitle = false
  }

  async function toggleShared() {
    const nextValue = !variant.is_shared
    shareBusy = true
    onVariantUpdated({ ...variant, is_shared: nextValue })
    try {
      const res = await variantApi.updateSharing(assetId, {
        [variant.id]: nextValue,
      })
      onVariantsUpdated(res.variants)
    } catch (error) {
      onVariantUpdated(variant)
      toastStore.show(
        error instanceof Error ? error.message : 'Failed to update sharing.',
        'error'
      )
    } finally {
      shareBusy = false
    }
  }
</script>

<div
  class="variant-card group"
  class:selected={isSelected}
  role="button"
  tabindex="0"
  onclick={onSelect}
  onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onSelect()}
  aria-pressed={isSelected}
>
  <div class="thumb-area {thumbBg}">
    {#if thumbUrl}
      <AssetThumbnail
        src={thumbUrl}
        contentType={thumbContentType}
        alt={variant.title}
        class="h-full w-full object-cover"
      />
    {:else if noPreview}
      <div class="thumb-placeholder">
        <ImageOff class="h-5 w-5" />
        <span class="thumb-label">{m.variant_no_preview()}</span>
      </div>
    {:else}
      <div class="thumb-placeholder">
        <div class="spinner"></div>
        <span class="thumb-label">{m.variant_thumbnail_generating()}</span>
      </div>
    {/if}

    <div class="menu-wrap">
      <a
        href={variantApi.fileUrl(assetId, variant.id)}
        download
        class="thumb-action top-2 right-10"
        aria-label={m.download()}
        onclick={(e) => e.stopPropagation()}
      >
        <Download class="h-3.5 w-3.5" />
      </a>

      {#if authStore.role !== 'viewer'}
        <button
          type="button"
          class="thumb-action top-2 right-2"
          aria-label="版本操作"
          onclick={(e) => {
            e.stopPropagation()
            menuOpen = !menuOpen
          }}
        >
          <MoreHorizontal class="h-3.5 w-3.5" />
        </button>
      {/if}
    </div>

    {#if menuOpen && authStore.role !== 'viewer'}
      <div class="variant-menu z-10">
        <button type="button" onclick={setAsThumbnail}>设为缩略图</button>
        <button
          type="button"
          onclick={() => {
            menuOpen = false
            onPromote()
          }}
        >
          Create new asset
        </button>
        {#if variant.type !== 'manual'}
          <button
            type="button"
            onclick={() => {
              menuOpen = false
              onReuse(variant)
            }}
          >
            Reuse
          </button>
        {/if}
        {#if variant.type === 'image_bg_remove' || variant.type === 'image_with_prompt'}
          <button type="button" onclick={rerunVariant}>Re-run</button>
        {/if}
        <button
          type="button"
          class="danger"
          onclick={() => {
            menuOpen = false
            showDeleteConfirm = true
          }}
        >
          {m.kb_delete()}
        </button>
      </div>
    {/if}
  </div>

  <div class="card-info">
    <div class="title-row">
      {#if editingTitle}
        <input
          bind:value={draftTitle}
          class="title-input"
          placeholder={variant.type}
          onkeydown={(e) => {
            e.stopPropagation()
            if (e.key === 'Enter') saveTitle()
            if (e.key === 'Escape') cancelEdit()
          }}
          onblur={() => {
            if (ignoreTitleBlur) {
              ignoreTitleBlur = false
              return
            }
            saveTitle()
          }}
        />
        <button
          type="button"
          class="icon-btn confirm"
          onpointerdown={() => (ignoreTitleBlur = true)}
          onclick={(event) => {
            event.stopPropagation()
            saveTitle()
          }}
          disabled={titleBusy}
          aria-label="Save title"
        >
          <Check class="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          class="icon-btn"
          onpointerdown={() => (ignoreTitleBlur = true)}
          onclick={(event) => {
            event.stopPropagation()
            cancelEdit()
          }}
          aria-label="Cancel edit"
        >
          <X class="h-3.5 w-3.5" />
        </button>
      {:else}
        <p class="card-name">{variant.title}</p>
        {#if authStore.role !== 'viewer'}
          <button
            type="button"
            class="icon-btn edit-btn"
            onclick={(event) => {
              event.stopPropagation()
              editingTitle = true
              draftTitle = variant.title
            }}
            aria-label="Edit title"
          >
            <Pencil class="h-3.5 w-3.5" />
          </button>
        {/if}
      {/if}
    </div>

    <div class="share-row">
      <span class="share-label" class:shared={variant.is_shared}
        >{variant.is_shared ? 'Shared' : 'Private'}</span
      >
      <button
        type="button"
        class="share-toggle"
        class:on={variant.is_shared}
        disabled={shareBusy || authStore.role === 'viewer'}
        onclick={(event) => {
          event.stopPropagation()
          toggleShared()
        }}
        aria-label={variant.is_shared ? 'Make private' : 'Share variant'}
        aria-pressed={variant.is_shared}
      >
        <span class="share-knob"></span>
      </button>
    </div>

    <p class="card-meta">
      {variant.size != null ? formatBytes(variant.size) + ' · ' : ''}{new Date(
        variant.created_at
      ).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      })}
    </p>
  </div>
</div>

<ConfirmModal
  bind:open={showDeleteConfirm}
  title={m.variant_delete_confirm()}
  items={[variant.type]}
  onConfirm={onDelete}
  onCancel={() => {
    showDeleteConfirm = false
  }}
/>

<style>
  .variant-card {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    cursor: pointer;
    transition: box-shadow 0.15s ease;
  }
  .variant-card:hover {
    box-shadow: 0 4px 12px -2px rgb(0 0 0 / 0.1);
  }
  .variant-card.selected {
    border-color: var(--accent-cta);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-cta) 40%, transparent);
  }
  .thumb-area {
    position: relative;
    display: flex;
    height: 160px;
    align-items: center;
    justify-content: center;
    overflow: visible;
  }
  .thumb-placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    color: var(--text-muted);
  }
  .thumb-label {
    font-size: 11px;
  }
  .spinner {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 2px solid var(--border);
    border-top-color: var(--accent-cta);
    animation: spin 0.8s linear infinite;
  }
  .thumb-action {
    position: absolute;
    display: flex;
    height: 28px;
    width: 28px;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    background: color-mix(in srgb, var(--bg-surface) 88%, transparent);
    backdrop-filter: blur(4px);
    color: var(--text-secondary);
    opacity: 0;
    transition: opacity 0.15s ease;
  }
  .thumb-action:hover {
    background: var(--bg-surface);
    color: var(--text-primary);
  }
  .variant-card:hover .thumb-action {
    opacity: 1;
  }
  .menu-wrap {
    position: absolute;
    inset: 0;
  }
  .variant-menu {
    position: absolute;
    top: 42px;
    right: 8px;
    display: grid;
    min-width: 170px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-surface);
    box-shadow: 0 16px 40px rgb(0 0 0 / 0.14);
    overflow: hidden;
  }
  .variant-menu button {
    padding: 9px 12px;
    text-align: left;
    font-size: 0.8125rem;
    color: var(--text-primary);
    transition: background 0.1s ease;
  }
  .variant-menu button:hover {
    background: var(--bg-hover);
  }
  .variant-menu button.danger {
    color: var(--accent-danger);
  }
  .card-info {
    display: grid;
    gap: 6px;
    padding: 12px;
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .card-name {
    margin: 0;
    flex: 1;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title-input {
    flex: 1;
    min-width: 0;
    font-size: 0.875rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 4px 8px;
    background: var(--bg-app);
    color: var(--text-primary);
    outline: none;
    transition:
      border-color 0.15s ease,
      box-shadow 0.15s ease;
  }
  .title-input:focus {
    border-color: var(--accent-cta);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-cta) 18%, transparent);
  }
  .icon-btn {
    flex-shrink: 0;
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 6px;
    color: var(--text-muted);
    transition:
      background 0.1s ease,
      color 0.1s ease;
  }
  .icon-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .icon-btn.confirm:hover {
    background: color-mix(in srgb, var(--accent-cta) 12%, transparent);
    color: var(--accent-cta);
  }
  .edit-btn {
    opacity: 0;
    transition:
      opacity 0.1s ease,
      background 0.1s ease,
      color 0.1s ease;
  }
  .variant-card:hover .edit-btn,
  .title-row:focus-within .edit-btn {
    opacity: 1;
  }
  .share-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.75rem;
  }
  .share-label {
    color: var(--text-muted);
    font-weight: 500;
    letter-spacing: 0.01em;
  }
  .share-label.shared {
    color: var(--accent-cta);
  }
  .share-toggle {
    position: relative;
    width: 32px;
    height: 18px;
    border-radius: 9px;
    background: var(--border);
    transition: background 0.2s ease;
    flex-shrink: 0;
  }
  .share-toggle:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .share-toggle.on {
    background: var(--accent-cta);
  }
  .share-toggle:focus-visible {
    outline: 2px solid var(--accent-cta);
    outline-offset: 2px;
  }
  .share-knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: white;
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.2);
    transition: transform 0.2s ease;
  }
  .share-toggle.on .share-knob {
    transform: translateX(14px);
  }
  .card-meta {
    margin: 0;
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
