<script lang="ts">
  import { Download } from '@lucide/svelte'
  import { type SharedVariant, publicShareApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { extFromMime } from '$lib/utils/mime'

  interface Props {
    shareId: string
    assetId: string
    variants: SharedVariant[]
    selectedIndex: number | null
    allowDownload: boolean
    getThumbUrl: (shareId: string, assetId: string, variantId: string) => string
    getDownloadUrl: (
      shareId: string,
      assetId: string,
      variantId: string
    ) => string
    authHeaders: () => Record<string, string>
    onSelect?: (index: number | null) => void
  }

  let {
    shareId,
    assetId,
    variants,
    selectedIndex = $bindable(null),
    allowDownload,
    getThumbUrl,
    getDownloadUrl,
    authHeaders,
    onSelect,
  }: Props = $props()

  let downloadingId = $state<string | null>(null)

  const selectedPosition = $derived(
    selectedIndex === null || variants.length === 0
      ? 0
      : Math.min(selectedIndex + 1, variants.length)
  )

  function variantCountLabel(count: number) {
    return count === 1
      ? m.share_variants_count({ count })
      : m.share_variants_count_plural({ count })
  }

  function formatSize(size: number | null | undefined): string {
    if (!size) return ''
    if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`
    return `${(size / (1024 * 1024)).toFixed(size < 10 * 1024 * 1024 ? 1 : 0)} MB`
  }

  function fallbackFilename(variant: SharedVariant): string {
    return `${variant.title}${extFromMime(variant.mime_type)}`
  }

  function parseContentDispositionFilename(
    header: string | null
  ): string | null {
    if (!header) return null

    const encodedMatch = header.match(/filename\*=([^;]+)/i)
    if (encodedMatch) {
      const value = encodedMatch[1].trim().replace(/^UTF-8''/i, '')
      try {
        return decodeURIComponent(value.replace(/^"|"$/g, ''))
      } catch {
        return value.replace(/^"|"$/g, '')
      }
    }

    const quotedMatch = header.match(/filename="([^"]+)"/i)
    if (quotedMatch) return quotedMatch[1]

    const plainMatch = header.match(/filename=([^;]+)/i)
    return plainMatch?.[1].trim() ?? null
  }

  function selectVariant(index: number) {
    selectedIndex = selectedIndex === index ? null : index
    onSelect?.(selectedIndex)
  }

  function handleCardKeydown(event: KeyboardEvent, index: number) {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    selectVariant(index)
  }

  async function downloadVariant(event: MouseEvent, variant: SharedVariant) {
    event.stopPropagation()
    downloadingId = variant.id
    try {
      const res = await publicShareApi.downloadFile(
        getDownloadUrl(shareId, assetId, variant.id),
        authHeaders()['X-Share-Token']
      )
      if (!res.ok) {
        toastStore.show(m.download_failed(), 'error')
        return
      }

      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download =
        parseContentDispositionFilename(
          res.headers.get('Content-Disposition')
        ) ?? fallbackFilename(variant)
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch {
      toastStore.show(m.download_failed(), 'error')
    } finally {
      downloadingId = null
    }
  }
</script>

{#if variants.length > 0}
  <section
    class="shared-variant-filmstrip"
    aria-label={m.share_variants_section_title()}
  >
    <div class="filmstrip-header">
      <div>
        <h3 class="variants-heading">{m.share_variants_section_title()}</h3>
        <p class="viewing">
          {m.share_variants_viewing({
            current: selectedPosition,
            total: variants.length,
          })}
        </p>
      </div>
      <span class="count-badge">{variantCountLabel(variants.length)}</span>
    </div>

    <div
      class="rail"
      role="listbox"
      aria-label={m.share_variants_section_title()}
    >
      {#each variants as variant, index}
        {@const selected = index === selectedIndex}
        {@const size = formatSize(variant.size)}
        <div
          class="card"
          class:selected
          role="option"
          aria-selected={selected}
          tabindex="0"
          onclick={() => selectVariant(index)}
          onkeydown={(event) => handleCardKeydown(event, index)}
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
            {#if allowDownload}
              <button
                class="download-btn"
                class:visible={selected}
                type="button"
                aria-label={downloadingId === variant.id
                  ? m.share_variants_downloading()
                  : m.share_variants_download()}
                title={m.share_variants_download()}
                disabled={downloadingId === variant.id}
                onclick={(event) => downloadVariant(event, variant)}
              >
                {#if downloadingId === variant.id}
                  <span class="spinner" aria-hidden="true"></span>
                {:else}
                  <Download class="h-3.5 w-3.5" aria-hidden="true" />
                {/if}
              </button>
            {/if}
          </div>
          <div class="meta">
            <p class="title" title={variant.title}>{variant.title}</p>
            <p class="sub">{variant.mime_type}{size ? ` · ${size}` : ''}</p>
          </div>
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .shared-variant-filmstrip {
    display: grid;
    gap: 12px;
    padding-top: 16px;
  }
  .filmstrip-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .variants-heading {
    margin: 0;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .viewing {
    margin: 3px 0 0;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }
  .count-badge {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    padding: 2px 8px;
    border: 1px solid var(--border-subtle);
    border-radius: 999px;
    background: var(--bg-elevated);
    color: var(--text-secondary);
    font-size: 0.6875rem;
    font-weight: 600;
    white-space: nowrap;
  }
  .rail {
    display: flex;
    gap: 10px;
    overflow-x: auto;
    overscroll-behavior-x: contain;
    scroll-snap-type: x proximity;
    padding: 2px 2px 8px;
  }
  .card {
    flex: 0 0 160px;
    display: grid;
    gap: 8px;
    scroll-snap-align: start;
    border: 1.5px solid var(--border);
    border-radius: 8px;
    padding: 7px;
    background: var(--bg-app);
    color: var(--text-primary);
    cursor: pointer;
    outline: none;
    transition:
      border-color 0.12s ease,
      background 0.12s ease,
      box-shadow 0.12s ease;
  }
  .card:hover,
  .card:focus-visible {
    background: var(--bg-hover);
    border-color: color-mix(
      in srgb,
      var(--accent-indigo, var(--accent-cta)) 45%,
      var(--border)
    );
  }
  .card.selected {
    border-color: var(--accent-indigo, var(--accent-cta));
    box-shadow: 0 0 0 2px
      color-mix(
        in srgb,
        var(--accent-indigo, var(--accent-cta)) 20%,
        transparent
      );
  }
  .thumb {
    position: relative;
    aspect-ratio: 4 / 3;
    width: 100%;
    overflow: hidden;
    border-radius: 6px;
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
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }
  .download-btn {
    position: absolute;
    right: 6px;
    bottom: 6px;
    display: inline-grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border: 1px solid rgb(255 255 255 / 0.72);
    border-radius: 999px;
    background: rgb(17 24 39 / 0.78);
    color: white;
    opacity: 0;
    transform: translateY(2px);
    transition:
      opacity 0.12s ease,
      transform 0.12s ease,
      background 0.12s ease;
  }
  .card:hover .download-btn,
  .card:focus-within .download-btn,
  .download-btn.visible {
    opacity: 1;
    transform: translateY(0);
  }
  .download-btn:hover:not(:disabled) {
    background: rgb(17 24 39 / 0.92);
  }
  .download-btn:disabled {
    cursor: wait;
  }
  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid rgb(255 255 255 / 0.45);
    border-top-color: white;
    border-radius: 999px;
    animation: spin 0.7s linear infinite;
  }
  .meta {
    min-width: 0;
  }
  .title {
    margin: 0;
    overflow: hidden;
    color: var(--text-primary);
    font-size: 0.8125rem;
    font-weight: 650;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    margin: 2px 0 0;
    overflow: hidden;
    color: var(--text-muted);
    font-size: 0.6875rem;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @media (max-width: 767px) {
    .shared-variant-filmstrip {
      display: none;
    }
  }
</style>
