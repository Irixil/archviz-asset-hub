<script lang="ts">
  import { type SharedVariant, publicShareApi } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { extFromMime } from '$lib/utils/mime'

  interface Props {
    shareId: string
    assetId: string
    variants: SharedVariant[]
    selectedVariantId?: string | null
    allowDownload?: boolean
    getThumbUrl: (shareId: string, assetId: string, variantId: string) => string
    getDownloadUrl: (
      shareId: string,
      assetId: string,
      variantId: string
    ) => string
    authHeaders: () => Record<string, string>
    onselect?: (variant: SharedVariant) => void
  }

  let {
    shareId,
    assetId,
    variants,
    selectedVariantId,
    allowDownload = true,
    getThumbUrl,
    getDownloadUrl,
    authHeaders,
    onselect,
  }: Props = $props()
  let downloadingId = $state<string | null>(null)

  async function downloadVariant(variant: SharedVariant) {
    downloadingId = variant.id
    try {
      const res = await publicShareApi.downloadFile(
        getDownloadUrl(shareId, assetId, variant.id),
        authHeaders()['X-Share-Token']
      )
      if (!res.ok) {
        toastStore.show('Download failed.', 'error')
        return
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = variant.title + extFromMime(variant.mime_type)
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch {
      toastStore.show('Download failed.', 'error')
    } finally {
      downloadingId = null
    }
  }
</script>

{#if variants.length > 0}
  <section class="shared-variants">
    <h3 class="variants-heading">{m.tab_variants()} ({variants.length})</h3>
    <div class="list">
      {#each variants as variant}
        <div
          class="row"
          class:selected={variant.id === selectedVariantId}
          role="button"
          tabindex="0"
          onclick={() => onselect?.(variant)}
          onkeydown={(e) =>
            (e.key === 'Enter' || e.key === ' ') && onselect?.(variant)}
        >
          <div class="thumb">
            {#if variant.thumbnail_url}
              <img
                src={getThumbUrl(shareId, assetId, variant.id)}
                alt={variant.title}
              />
            {:else}
              <div class="fallback">{variant.type}</div>
            {/if}
          </div>
          <div class="meta">
            <p class="meta-title">{variant.title}</p>
            <span class="meta-sub"
              >{variant.mime_type}{variant.size
                ? ` · ${Math.round(variant.size / 1024)} KB`
                : ''}</span
            >
          </div>
          {#if allowDownload}
            <Button
              variant="secondary"
              size="sm"
              loading={downloadingId === variant.id}
              onclick={(event) => {
                event.stopPropagation()
                downloadVariant(variant)
              }}
            >
              {m.download()}
            </Button>
          {/if}
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .shared-variants {
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  @media (min-width: 768px) {
    .shared-variants {
      display: none;
    }
  }
  .variants-heading {
    margin: 0 0 12px;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .list {
    display: grid;
    gap: 2px;
  }
  .row {
    display: grid;
    grid-template-columns: 48px 1fr auto;
    gap: 12px;
    align-items: center;
    padding: 8px;
    border-radius: 6px;
    transition: background 0.1s ease;
  }
  .row:hover {
    background: var(--bg-hover);
  }
  .row.selected {
    background: color-mix(in srgb, var(--accent-cta) 12%, transparent);
    outline: 1.5px solid color-mix(in srgb, var(--accent-cta) 40%, transparent);
  }
  .thumb {
    width: 48px;
    height: 48px;
    flex-shrink: 0;
    border-radius: 6px;
    overflow: hidden;
    background: var(--bg-elevated);
  }
  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .fallback {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    color: var(--text-muted);
    font-size: 0.625rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }
  .meta {
    min-width: 0;
  }
  .meta-title {
    margin: 0;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .meta-sub {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-top: 2px;
  }
</style>
