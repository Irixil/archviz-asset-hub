<script lang="ts">
  import { longPress } from '$lib/actions/longPress'
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api'
  import { customFieldsStore } from '$lib/stores/customFields.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { Check, File, Loader, Play, TriangleAlert } from '@lucide/svelte'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'
  import { m } from '$lib/paraglide/messages'
  import { mount, unmount } from 'svelte'
  import DragGhost from '$lib/components/DragGhost.svelte'
  import AssetThumbnail from '$lib/components/asset/AssetThumbnail.svelte'
  import {
    assetFormatLabel,
    assetTypeLabel,
    canGenerateThumbnail,
    setAssetDragData,
  } from '$lib/utils/assetFiles'

  interface Props {
    asset: Asset
    class?: string
    zoom?: number
    gridMode?: 'compact' | 'spaced' | 'table'
    onclick: (e: MouseEvent) => void
    /** Set to true when this asset was just uploaded and may be missing required fields */
    requiresFields?: boolean
    /** All selected IDs when this card is part of a multi-selection drag; empty for solo drag */
    draggedIds?: string[]
    isSelected?: boolean
    onLongPress?: () => void
  }

  let {
    asset,
    class: _extraClass = '',
    zoom = 5,
    gridMode = 'compact',
    onclick,
    requiresFields = false,
    draggedIds = [],
    isSelected = false,
    onLongPress,
  }: Props = $props()

  const hasRequiredFields = $derived(
    customFieldsStore.assetFields.some((f) => f.required)
  )
  const showRequiredNudge = $derived(requiresFields && hasRequiredFields)
  const category = $derived(mimeCategory(asset.mime_type))
  const isProcessing = $derived(
    canGenerateThumbnail(asset.original_filename, asset.mime_type) &&
      !asset.thumbnail_key
  )
  const assetType = $derived(
    assetTypeLabel(asset.original_filename, asset.mime_type)
  )
  const assetFormat = $derived(
    assetFormatLabel(asset.original_filename, asset.mime_type)
  )
  const original_basename = $derived(
    asset.original_filename?.replace(/\.[^.]+$/, '') || asset.original_filename
  )
  const previewRatio = $derived(
    asset.width && asset.height ? `${asset.width} / ${asset.height}` : '4 / 3'
  )
  const dimensions = $derived(
    asset.width && asset.height ? `${asset.width}×${asset.height}` : null
  )

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('zh-CN')
  }
</script>

<div class="flex w-full flex-col {gridMode === 'spaced' ? 'gap-2' : ''}">
  <button
    type="button"
    draggable="true"
    use:longPress={viewportStore.isTouch && onLongPress
      ? {
          onLongPress,
        }
      : {
          onLongPress: () => {},
        }}
    class="asset-card group relative flex w-full flex-col overflow-hidden rounded-md border border-[var(--border-default)] bg-[var(--bg-surface)] text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--focus-ring)] focus-visible:ring-offset-2 {isSelected
      ? 'selected'
      : ''}"
    title={`拖至 3ds Max、Photoshop 或文件夹使用｜${asset.original_filename}`}
    aria-label={`${asset.original_filename}，${assetType}，可拖出使用`}
    onclick={(e) => onclick(e)}
    ondragstart={(e) => {
      const dragMode = setAssetDragData(e, asset, assetApi.fileUrl(asset.id))
      if (dragMode === 'native') return

      // Transparent 1×1 pixel suppresses the browser's default drag ghost
      const pixel = document.createElement('canvas')
      pixel.width = pixel.height = 1
      e.dataTransfer?.setDragImage(pixel, 0, 0)

      // Mount the visual ghost that follows the cursor
      const ids = draggedIds.length > 0 ? draggedIds : [asset.id]
      const wrapper = document.createElement('div')
      wrapper.style.cssText =
        'position:fixed;pointer-events:none;z-index:9999;top:0;left:0;transform:translate(-50%,-50%)'
      document.body.appendChild(wrapper)
      const app = mount(DragGhost, {
        target: wrapper,
        props: { assetIds: ids },
      })

      const onDragOver = (ev: DragEvent) => {
        wrapper.style.left = ev.clientX + 'px'
        wrapper.style.top = ev.clientY + 'px'
      }
      document.addEventListener('dragover', onDragOver)

      e.currentTarget?.addEventListener(
        'dragend',
        () => {
          document.removeEventListener('dragover', onDragOver)
          unmount(app)
          wrapper.remove()
        },
        { once: true }
      )
    }}
  >
    <!-- stack effect -->
    <div
      class="card-shadow {asset.variant_count > 0
        ? 'block'
        : 'hidden'} absolute top-[-3px] right-[-3px] z-0 h-full w-full rounded-lg border border-gray-300 [transition:top_150ms_ease,right_150ms_ease] dark:border-gray-700"
    ></div>
    <div
      class="card-shadow {asset.variant_count > 2
        ? 'block'
        : 'hidden'} absolute top-[-3px] right-[-3px] z-0 h-full w-full rounded-lg border border-gray-300 [transition:top_150ms_ease,right_150ms_ease] dark:border-gray-700"
    ></div>

    <!-- Thumbnail area -->
    <div
      class="damask-texture relative w-full overflow-hidden rounded-t-lg {ASSET_BACKGROUND_COLORS[
        category
      ]}"
      style="aspect-ratio: {previewRatio}"
    >
      <!-- Required fields nudge -->
      {#if showRequiredNudge}
        <div
          class="absolute top-2 right-2 z-10 flex items-center gap-1 rounded-sm bg-[var(--accent-cta)] px-1.5 py-1 text-white shadow"
          title="缺少必填字段"
        >
          <TriangleAlert class="h-3 w-3" />
          <span class="text-xs leading-none font-semibold">{m.fields()}</span>
        </div>
      {:else if asset.version_count > 1}
        <div
          class="absolute top-2 right-2 z-10 rounded-sm bg-[#27312d]/85 px-1.5 py-1 text-white backdrop-blur-sm"
          title="{asset.version_count} 个版本"
        >
          <span class="text-xs leading-none font-bold"
            >v{asset.version_count}</span
          >
        </div>
      {/if}

      {#if asset.thumbnail_key && (category === 'image' || category === 'video' || category === 'audio' || category === 'document')}
        <AssetThumbnail
          src={assetApi.thumbUrl(asset.id)}
          contentType={asset.thumbnail_content_type
            ? asset.thumbnail_content_type
            : asset.thumbnail_key?.includes('.mp4')
              ? 'video/mp4'
              : 'image/jpeg'}
          alt={asset.original_filename}
          class="h-full w-full object-cover"
          assetId={asset.id}
        />
      {:else}
        <!-- Centered icon -->
        <div class="flex h-full items-center justify-center">
          <div
            class="flex h-12 w-12 items-center justify-center rounded-md border border-black/10 bg-white/45"
          >
            {#if category === 'video' || category === 'audio'}
              <Play class="h-7 w-7 text-[var(--text-secondary)]" />
            {:else}
              <File class="h-7 w-7 text-[var(--text-secondary)]" />
            {/if}
          </div>
        </div>
      {/if}

      {#if isProcessing}
        <div
          class="absolute inset-0 flex items-center justify-center bg-black/25"
        >
          <div class="flex flex-col items-center gap-1.5 text-white">
            <Loader class="h-5 w-5 animate-spin" />
            <span class="text-sm font-medium">{m.processing()}</span>
          </div>
        </div>
      {/if}
    </div>

    <!-- Info -->
    <div
      class="flex min-w-0 flex-col gap-1.5 border-t border-[var(--border-subtle)] px-3 py-2.5"
    >
      <p
        class="truncate text-sm font-semibold text-[var(--text-primary)]"
        title={asset.original_filename}
      >
        {original_basename}
      </p>
      <div
        class="flex min-w-0 items-center gap-1.5 text-[11px] text-[var(--text-muted)]"
      >
        <span
          class="shrink-0 rounded-sm border border-[var(--border-default)] bg-[var(--bg-app)] px-1.5 py-0.5 font-bold tracking-wide text-[var(--text-secondary)]"
          >{assetFormat}</span
        >
        <span class="truncate">{assetType}</span>
        {#if asset.classification_status === 'needs_review'}
          <span
            class="ml-auto shrink-0 rounded-sm bg-amber-100 px-1.5 py-0.5 font-medium text-amber-800"
            title="打开素材详情可重新分类">待分类</span
          >
        {/if}
      </div>
      <div
        class="flex items-center justify-between gap-2 text-xs text-[var(--text-muted)] tabular-nums"
      >
        <span>{dimensions ?? formatBytes(asset.size)}</span>
        <span
          >{dimensions
            ? formatBytes(asset.size)
            : formatDate(asset.created_at)}</span
        >
      </div>
      {#if zoom >= 6 && asset.tags && asset.tags.length > 0}
        <div class="flex flex-wrap gap-1">
          {#each asset.tags.slice(0, 2) as tag}
            <span
              class="rounded-sm bg-[var(--bg-elevated)] px-1.5 py-0.5 text-[11px] text-[var(--text-secondary)]"
              >{tag}</span
            >
          {/each}
        </div>
      {/if}
    </div>

    {#if isSelected}
      <div
        class="asset-selection-indicator pointer-events-none absolute top-1.5 right-1.5 z-20 flex h-11 w-11 items-start justify-end p-1"
      >
        <span
          class="flex h-5 w-5 items-center justify-center rounded-sm bg-[var(--accent-cta)] shadow"
        >
          <Check class="h-3 w-3 text-white" />
        </span>
      </div>
    {/if}
  </button>
</div>

<style>
  .asset-card {
    transition:
      transform 140ms ease-out,
      border-color 140ms ease-out,
      box-shadow 140ms ease-out;
  }
  :global(.group:hover) .asset-card {
    transform: translateY(-1px);
    border-color: color-mix(
      in oklab,
      var(--text-primary) 32%,
      var(--border-default)
    );
    box-shadow: 0 8px 18px rgb(24 30 27 / 0.12);
    position: relative;
    z-index: 10;
  }
  @media (prefers-reduced-motion: reduce) {
    .asset-card {
      transition: none !important;
      transform: none !important;
    }
  }

  :global(.asset-card img) {
    pointer-events: none;
  }

  .asset-card.selected {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }

  .card-shadow + .card-shadow {
    top: -6px;
    right: -6px;
  }
  .asset-card:hover .card-shadow {
    top: -4px;
    right: -4px;
  }
  .asset-card:hover .card-shadow + .card-shadow {
    top: -8px;
    right: -8px;
  }

  :global(.asset-card--opening) {
    border-color: var(--accent);
  }

  @media (prefers-reduced-motion: reduce) {
    :global(.asset-card--opening) {
      animation: none;
    }
  }
</style>
