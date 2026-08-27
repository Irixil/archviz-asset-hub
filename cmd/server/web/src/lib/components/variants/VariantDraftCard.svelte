<script lang="ts">
  import { X } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    assetId: string
    nonce: string
    previewUrl: string
    expiresAt: string
    errorMsg: string
    phase: 'generating' | 'ready' | 'error' | 'queued'
    isCommitting: boolean
    mediaKind?: 'image' | 'video' | 'audio' | 'other'
    mediaUrl?: string
    onKeep: () => void
    onDiscard: () => void
  }

  let {
    assetId: _assetId,
    nonce: _nonce,
    previewUrl,
    expiresAt,
    errorMsg,
    phase,
    isCommitting,
    mediaKind = 'image',
    mediaUrl,
    onKeep,
    onDiscard,
  }: Props = $props()

  let lightboxOpen = $state(false)
  let dialogEl = $state<HTMLDialogElement | undefined>(undefined)

  const expiresInHours = $derived(
    !expiresAt
      ? null
      : (() => {
          const diff = new Date(expiresAt).getTime() - Date.now()
          return diff <= 0 ? null : Math.ceil(diff / (1000 * 60 * 60))
        })()
  )

  const API_BASE = import.meta.env.VITE_API_URL ?? ''
  const fullPreviewUrl = $derived(
    mediaUrl || (previewUrl ? `${API_BASE}${previewUrl}` : '')
  )

  $effect(() => {
    if (dialogEl && !dialogEl.open) {
      dialogEl.showModal()
    }
  })
</script>

<div class="draft-card">
  {#if phase === 'generating'}
    <div class="preview-skeleton" aria-label={m.variants_draft_generating()}>
      <div class="skeleton-shimmer"></div>
    </div>
    <div class="card-footer">
      <span class="status-text">{m.variants_draft_generating()}</span>
    </div>
  {:else if phase === 'queued'}
    <div
      class="preview-skeleton"
      aria-label={m.variants_draft_running_full_job()}
    >
      <div class="skeleton-shimmer"></div>
    </div>
    <div class="card-footer">
      <span class="status-text">{m.variants_draft_running_full_job()}</span>
    </div>
  {:else if phase === 'error'}
    <div class="preview-error">
      <X size={28} class="error-icon" />
      <p class="error-text">{errorMsg}</p>
    </div>
    <div class="card-footer">
      <button type="button" class="btn-discard" onclick={onDiscard}>
        {m.variants_draft_discard()}
      </button>
    </div>
  {:else}
    {#if mediaKind === 'video'}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video src={fullPreviewUrl} controls class="preview-img"></video>
    {:else if mediaKind === 'audio'}
      <audio src={fullPreviewUrl} controls class="preview-audio"></audio>
    {:else}
      <button
        type="button"
        class="preview-button"
        onclick={() => (lightboxOpen = true)}
        aria-label="Open full-size preview"
      >
        <img
          src={fullPreviewUrl}
          alt="Draft preview"
          class="preview-img"
          loading="lazy"
        />
      </button>
    {/if}

    {#if expiresInHours !== null}
      <p class="expires-hint">
        {m.variants_draft_expires_in({ hours: String(expiresInHours) })}
      </p>
    {/if}

    <div class="card-footer">
      <button
        type="button"
        class="btn-discard"
        disabled={isCommitting}
        onclick={onDiscard}
      >
        {m.variants_draft_discard()}
      </button>
      <button
        type="button"
        class="btn-keep"
        disabled={isCommitting}
        onclick={onKeep}
      >
        {isCommitting
          ? m.variants_draft_keeping()
          : m.variants_draft_keep_one()}
      </button>
    </div>
  {/if}
</div>

{#if lightboxOpen && fullPreviewUrl}
  <dialog
    bind:this={dialogEl}
    class="draft-lightbox"
    onclick={() => (lightboxOpen = false)}
    onkeydown={(e) => e.key === 'Escape' && (lightboxOpen = false)}
  >
    <img
      src={fullPreviewUrl}
      alt="Draft preview full size"
      class="lightbox-img"
    />
  </dialog>
{/if}

<style>
  .draft-card {
    display: flex;
    flex-flow: column;
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    background: var(--bg-surface);
  }

  .preview-skeleton {
    position: relative;
    width: 100%;
    height: 200px;
    background: var(--bg-muted, oklch(96% 0 0));
    overflow: hidden;
  }

  :global(.dark) .preview-skeleton {
    background: oklch(25% 0 0);
  }

  .skeleton-shimmer {
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent 0%,
      oklch(90% 0 0 / 0.6) 50%,
      transparent 100%
    );
    animation: shimmer 1.4s infinite;
    background-size: 200% 100%;
  }

  :global(.dark) .skeleton-shimmer {
    background: linear-gradient(
      90deg,
      transparent 0%,
      oklch(35% 0 0 / 0.6) 50%,
      transparent 100%
    );
    background-size: 200% 100%;
    animation: shimmer 1.4s infinite;
  }

  @keyframes shimmer {
    0% {
      background-position: -200% 0;
    }
    100% {
      background-position: 200% 0;
    }
  }

  .preview-error {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px 16px;
    min-height: 120px;
    color: var(--text-muted);
  }

  .error-text {
    font-size: 0.8125rem;
    color: var(--text-muted);
    text-align: center;
    max-width: 240px;
  }

  .preview-button {
    display: block;
    width: 100%;
    padding: 0;
    border: none;
    background: none;
    cursor: zoom-in;
    overflow: hidden;
  }

  .preview-audio {
    width: 100%;
    margin: 16px 12px;
  }

  .preview-img {
    display: block;
    width: 100%;
    max-height: 360px;
    object-fit: contain;
    background: oklch(96% 0 0);
  }

  :global(.dark) .preview-img {
    background: oklch(18% 0 0);
  }

  .expires-hint {
    margin-top: auto;
    font-size: 0.75rem;
    color: var(--text-muted);
    padding: 6px 12px 0;
  }

  .card-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 10px 12px;
    border-top: 1px solid var(--border-subtle);
  }

  .status-text {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }

  .btn-discard {
    padding: 5px 12px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.8125rem;
    cursor: pointer;
    transition:
      background 120ms ease,
      color 120ms ease;
  }

  .btn-discard:not(:disabled):hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .btn-discard:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .btn-keep {
    padding: 5px 14px;
    border-radius: 6px;
    border: none;
    background: var(--accent-cta);
    color: #fff;
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: opacity 120ms ease;
  }

  .btn-keep:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .draft-lightbox {
    margin: auto;
    border: none;
    border-radius: 8px;
    padding: 0;
    background: transparent;
    max-width: min(90vw, 1200px);
    max-height: 90vh;
    cursor: zoom-out;
  }

  .draft-lightbox::backdrop {
    background: oklch(0% 0 0 / 0.8);
  }

  .lightbox-img {
    display: block;
    max-width: 100%;
    max-height: 90vh;
    object-fit: contain;
    border-radius: 8px;
    cursor: default;
  }
</style>
