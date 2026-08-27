<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { uploadsStore } from '$lib/stores/uploads.svelte'
  import AssetCard from '$lib/components/asset/AssetCard.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import AssetIcon from '$lib/components/asset/AssetIcon.svelte'
  import OnboardingScreen from '$lib/components/OnboardingScreen.svelte'
  import {
    CATEGORY_ICON_BG,
    CATEGORY_LABELS,
    CATEGORY_ORDER,
  } from '$lib/stores/assetView'
  import { stackStore } from '$lib/stores/stack.svelte'
  import { Check, CloudUpload, Inbox, Loader } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'
  import { statusBarStore } from '$lib/stores/bottomStatusBar.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { onDestroy, onMount } from 'svelte'
  import AssetStateStackable from '$lib/components/asset/AssetStateStackable.svelte'
  import AssetThumbnail from '$lib/components/asset/AssetThumbnail.svelte'
  import {
    assetFormatLabel,
    assetTypeLabel,
    setAssetDragData,
  } from '$lib/utils/assetFiles'

  const fmt = new Intl.DateTimeFormat(undefined, {
    month: 'long',
    year: 'numeric',
  })

  const SIZE_BUCKETS: { label: string; min: number }[] = [
    { label: '> 1 GB', min: 1_000_000_000 },
    { label: '> 500 MB', min: 500_000_000 },
    { label: '> 100 MB', min: 100_000_000 },
    { label: '> 10 MB', min: 10_000_000 },
    { label: '> 1 MB', min: 1_000_000 },
    { label: '< 1 MB', min: 0 },
  ]

  type Props = {
    seenSplashScreen: boolean
    sort: 'mimetype' | 'created_at' | 'size' | 'taken_at'
    gridMode: 'compact' | 'spaced' | 'table'
    isDraggingFiles: boolean
    emptyProjectName?: string | null
    mainEl?: HTMLElement
    onCardClick: (asset: Asset, index: number, event: MouseEvent) => void
    onDismissOnboarding: () => void
    onDragOver: (e: DragEvent) => void
    onDragLeave: (e: DragEvent) => void
    onDrop: (e: DragEvent) => void
    onMouseDown: (e: MouseEvent) => void
    onSearchEntireWorkspace: () => void
  }

  let {
    seenSplashScreen,
    sort,
    gridMode = 'compact',
    isDraggingFiles,
    emptyProjectName = null,
    mainEl = $bindable(),
    onCardClick,
    onDismissOnboarding,
    onDragOver,
    onDragLeave,
    onDrop,
    onMouseDown,
    onSearchEntireWorkspace,
  }: Props = $props()

  let sentinel = $state<HTMLDivElement | undefined>(undefined)

  let zoom = $derived(statusBarStore.zoom)
  let maxZoom = $derived(statusBarStore.maxZoom)
  const sourceAsset = $derived.by(() => {
    const meta = assetsStore.similarity
    if (!meta) return null
    return (
      assetsStore.assets.find((asset) => asset.id === meta.anchor_asset_id) ??
      null
    )
  })
  const displayAssets = $derived.by(() =>
    sourceAsset
      ? assetsStore.assets.filter((asset) => asset.id !== sourceAsset.id)
      : assetsStore.assets
  )

  const assetsByCategory = $derived.by(() => {
    if (sort !== 'mimetype') return {} as Record<string, Asset[]>
    const result: Record<string, Asset[]> = {}
    for (const asset of displayAssets) {
      const cat = mimeCategory(asset.mime_type)
      ;(result[cat] ??= []).push(asset)
    }
    return result
  })

  const assetsBySize = $derived.by(() => {
    if (sort !== 'size') return []
    const groups: { label: string; assets: Asset[] }[] = []
    for (let i = 0; i < SIZE_BUCKETS.length; i++) {
      const { label, min } = SIZE_BUCKETS[i]
      const max = i === 0 ? Infinity : SIZE_BUCKETS[i - 1].min
      const assets = displayAssets.filter((a) => a.size >= min && a.size < max)
      if (assets.length > 0) groups.push({ label, assets })
    }
    return groups
  })

  const assetsByMonth = $derived.by(() => {
    if (sort === 'created_at') {
      const groups: { label: string; assets: Asset[] }[] = []
      for (const asset of displayAssets) {
        const label = fmt.format(new Date(asset.created_at))
        if (groups.length === 0 || groups[groups.length - 1].label !== label) {
          groups.push({ label, assets: [asset] })
        } else {
          groups[groups.length - 1].assets.push(asset)
        }
      }
      return groups
    }
    return []
  })

  $effect(() => {
    void assetsStore.resetDone // re-create observer after each reset so it fires immediately if sentinel is already visible
    if (!sentinel) return
    const el = sentinel
    const observer = new IntersectionObserver(
      async (entries) => {
        if (
          entries[0].isIntersecting &&
          assetsStore.nextCursor &&
          !assetsStore.loading
        ) {
          await assetsStore.load()
        }
      },
      { rootMargin: '200px' }
    )
    observer.observe(el)
    return () => observer.disconnect()
  })

  $effect(() => {
    if (selectionStore.selectedIds.size > 0) {
      statusBarStore.slot1 = m.selected_assets({
        count: selectionStore.selectedIds.size,
      })
    } else {
      statusBarStore.slot1 = m.assets_count({
        count: assetsStore.assets.length,
      })
    }
  })

  onMount(() => {
    statusBarStore.showZoom = gridMode !== 'table' && !viewportStore.isMobile
  })

  onDestroy(() => {
    statusBarStore.showZoom = false
    statusBarStore.slot1 = null
  })

  $effect(() => {
    statusBarStore.showZoom = gridMode !== 'table' && !viewportStore.isMobile
  })

  function handleTableKeydown(e: KeyboardEvent, asset: Asset, index: number) {
    if (e.key !== 'Enter' && e.key !== ' ') return
    e.preventDefault()
    onCardClick(asset, index, new MouseEvent('click'))
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<main
  bind:this={mainEl}
  class="library-content relative flex-1 overflow-y-auto bg-[var(--bg-asset-grid)] px-3 py-3 sm:px-4 sm:py-4 {stackStore.count >
  0
    ? 'pb-20'
    : ''}"
  ondragover={onDragOver}
  ondragleave={onDragLeave}
  ondrop={onDrop}
  onmousedown={onMouseDown}
>
  {#snippet assetCardGrid(assets: readonly Asset[], withSentinel = false)}
    <div
      class="grid pt-2 {gridMode == 'compact'
        ? viewportStore.isMobile
          ? 'gap-3'
          : 'gap-3.5'
        : viewportStore.isMobile
          ? 'gap-6'
          : 'gap-5'}"
      style="grid-template-columns: repeat({viewportStore.isMobile
        ? Math.min(1 + maxZoom - Math.floor(zoom), 2)
        : 1 + maxZoom - Math.floor(zoom)}, minmax(0, 1fr))"
    >
      {#each assets as asset (asset.id)}
        {@const globalIndex = assetsStore.assets.indexOf(asset)}
        <div
          class="group relative {selectionStore.selectedIds.has(asset.id)
            ? 'is-selected'
            : ''}"
          data-asset-id={asset.id}
        >
          <AssetCard
            {asset}
            {zoom}
            {gridMode}
            draggedIds={selectionStore.selectedIds.has(asset.id)
              ? [...selectionStore.selectedIds]
              : []}
            requiresFields={uploadsStore.recentlyUploadedIds.has(asset.id)}
            isSelected={selectionStore.selectedIds.has(asset.id)}
            onclick={(e) => onCardClick(asset, globalIndex, e)}
            onLongPress={() => {
              selectionStore.toggle(asset, globalIndex)
            }}
          />

          <!-- Stack indicator -->
          {#if stackStore.active}
            <AssetStateStackable {asset} />
          {/if}
        </div>
      {/each}
      {#if withSentinel}
        <div class="grid-sentinel opacity-0" bind:this={sentinel}>&nbsp;</div>
      {/if}
    </div>
  {/snippet}

  {#snippet assetTable(assets: readonly Asset[], withSentinel = false)}
    <table
      class="asset-table w-full border-separate border-spacing-0 pt-2 text-left text-sm"
    >
      <thead>
        <tr
          class="text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
        >
          <th class="w-8 py-2 pr-4 pl-1"></th>
          <th class="py-2 pr-4">名称</th>
          <th class="py-2 pr-4">类型</th>
          <th class="py-2 pr-4">大小</th>
          <th class="py-2 pr-4">尺寸</th>
          <th class="py-2">添加日期</th>
        </tr>
      </thead>
      <tbody>
        {#each assets as asset (asset.id)}
          {@const globalIndex = assetsStore.assets.indexOf(asset)}
          {@const category = mimeCategory(asset.mime_type)}
          {@const isSelected = selectionStore.selectedIds.has(asset.id)}
          <tr
            data-asset-id={asset.id}
            draggable="true"
            tabindex="0"
            aria-selected={isSelected}
            title="拖至 3ds Max、Photoshop 或文件夹使用"
            class="group cursor-pointer border-t border-[var(--border-subtle)] transition-colors {isSelected
              ? 'selected-row bg-[var(--accent-soft)]'
              : 'even:bg-[var(--bg-stripe)] hover:bg-[var(--bg-hover)]'}"
            onclick={(e) =>
              onCardClick(asset, globalIndex, e as unknown as MouseEvent)}
            onkeydown={(e) => handleTableKeydown(e, asset, globalIndex)}
            ondragstart={(e) =>
              setAssetDragData(e, asset, assetApi.fileUrl(asset.id))}
          >
            <td class="w-8 py-2 pr-4 pl-1">
              <div
                class="relative flex h-8 w-8 items-center justify-center overflow-hidden rounded {CATEGORY_ICON_BG[
                  category
                ].light} {CATEGORY_ICON_BG[category].dark}"
              >
                {#if category === 'image' || category === 'video' || category === 'audio' || category === 'document'}
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
                  <AssetIcon {category} class="h-4 w-4" />
                {/if}
                {#if isSelected}
                  <div
                    class="absolute inset-0 flex items-center justify-center rounded bg-[var(--accent-cta)]/85"
                  >
                    <Check class="h-3.5 w-3.5 text-white" />
                  </div>
                {/if}
              </div>
            </td>
            <td class="max-w-xs py-2 pr-4">
              <span
                class="block truncate font-medium text-[var(--text-primary)]"
                >{asset.original_filename}</span
              >
              {#if asset.tags.length > 0}
                <span class="text-xs text-[var(--text-muted)]"
                  >{asset.tags.join(', ')}</span
                >
              {/if}
            </td>
            <td class="py-2 pr-4 text-[var(--text-muted)]">
              {assetTypeLabel(asset.original_filename, asset.mime_type)} ·
              {assetFormatLabel(asset.original_filename, asset.mime_type)}
            </td>
            <td class="py-2 pr-4 whitespace-nowrap text-[var(--text-muted)]"
              >{formatBytes(asset.size)}</td
            >
            <td class="py-2 pr-4 whitespace-nowrap text-[var(--text-muted)]">
              {#if asset.width && asset.height}{asset.width}×{asset.height}{:else}—{/if}
            </td>
            <td class="py-2 whitespace-nowrap text-[var(--text-muted)]">
              {new Date(asset.created_at).toLocaleDateString('zh-CN')}
            </td>
          </tr>
        {/each}
        {#if withSentinel}
          <tr class="grid-sentinel opacity-0" bind:this={sentinel}
            ><td>&nbsp;</td></tr
          >
        {/if}
      </tbody>
    </table>
  {/snippet}

  {#if isDraggingFiles}
    <div
      class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-[var(--accent-soft)] ring-2 ring-[var(--accent)] ring-inset backdrop-blur-sm"
    >
      <div class="flex flex-col items-center gap-2 text-[var(--accent-text)]">
        <CloudUpload class="h-10 w-10" />
        <p class="text-sm font-medium">{m.drop_to_upload()}</p>
      </div>
    </div>
  {/if}

  {#if assetsStore.initialLoad}
    <GridSkeleton x={7} y={3} {zoom} {maxZoom} />
  {:else if assetsStore.assets.length === 0}
    {#if !seenSplashScreen}
      <OnboardingScreen
        onDismiss={onDismissOnboarding}
        projectName={emptyProjectName}
      />
    {:else}
      <EmptyState
        title={assetsStore.query
          ? `没有找到“${assetsStore.query}”`
          : emptyProjectName
            ? `“${emptyProjectName}”暂无素材`
            : m.no_assets_yet()}
        description={assetsStore.query
          ? m.search_try_different()
          : emptyProjectName
            ? '上传项目文件，或从素材库把已有素材归入这个项目。'
            : m.upload_get_started()}
      >
        {#snippet icon()}<Inbox class="h-16 w-16" />{/snippet}
        {#snippet action()}
          {#if !assetsStore.query}
            <button
              type="button"
              class="min-h-10 rounded-md bg-[var(--accent-cta)] px-3.5 text-sm font-semibold text-white hover:bg-[var(--accent-cta-hover)]"
              onclick={() =>
                document
                  .querySelector<HTMLElement>('[data-upload-trigger]')
                  ?.click()}
              >{emptyProjectName ? '向项目添加素材' : '上传素材'}</button
            >
          {/if}
        {/snippet}
      </EmptyState>
    {/if}
  {:else if sourceAsset && displayAssets.length === 0}
    <div class="mt-8 flex flex-col items-center gap-3 text-center">
      <p class="text-sm font-medium text-[var(--text-secondary)]">
        {m.similarity_empty_in_context()}
      </p>
      <button
        type="button"
        class="rounded-md border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-1.5 text-sm font-medium text-[var(--text-primary)] transition-colors hover:bg-[var(--bg-hover)]"
        onclick={onSearchEntireWorkspace}
      >
        {m.similarity_search_workspace()}
      </button>
    </div>
  {:else if gridMode === 'table'}
    {@render assetTable(displayAssets, true)}
    <div class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else if sort === 'mimetype'}
    {#each CATEGORY_ORDER as cat, catIndex}
      {@const group = assetsByCategory[cat] ?? []}
      {#if group.length > 0}
        <div class="mb-10">
          <div
            class="sticky top-[-25px] z-10 flex items-center gap-3 bg-[var(--bg-app)] p-2"
          >
            <AssetIcon
              category={cat}
              class="h-7 w-7 items-center justify-center rounded-lg {CATEGORY_ICON_BG[
                cat
              ].light} {CATEGORY_ICON_BG[cat].dark}"
            />
            <h2 class="text-sm font-semibold text-[var(--text-primary)]">
              {CATEGORY_LABELS[cat]}
            </h2>
            <span class="text-xs text-[var(--text-muted)] tabular-nums"
              >{group.length}</span
            >
          </div>
          {@render assetCardGrid(group, catIndex >= CATEGORY_ORDER.length - 1)}
        </div>
      {/if}
    {/each}
    <div class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else if sort === 'taken_at'}
    {@render assetCardGrid(displayAssets, true)}
    <div class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else if sort === 'created_at'}
    {#each assetsByMonth as group, groupIndex}
      <div class="mb-10">
        <div
          class="sticky top-[-25px] z-10 flex items-center gap-3 bg-[var(--bg-app)] p-2"
        >
          <h2 class="text-sm font-semibold text-[var(--text-primary)]">
            {group.label}
          </h2>
          <span class="text-xs text-[var(--text-muted)] tabular-nums"
            >{group.assets.length}</span
          >
        </div>
        {@render assetCardGrid(
          group.assets,
          groupIndex >= assetsByMonth.length - 1
        )}
      </div>
    {/each}
    <div class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else}
    {#each assetsBySize as group, groupIndex}
      <div class="mb-10">
        <div
          class="sticky top-[-25px] z-10 flex items-center gap-3 bg-[var(--bg-app)] p-2"
        >
          <h2 class="text-sm font-semibold text-[var(--text-primary)]">
            {group.label}
          </h2>
          <span class="text-xs text-[var(--text-muted)] tabular-nums"
            >{group.assets.length}</span
          >
        </div>
        {@render assetCardGrid(
          group.assets,
          groupIndex >= assetsBySize.length - 1
        )}
      </div>
    {/each}
    <div class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {/if}
</main>

<style>
  :global(table.asset-table) {
    user-select: none;
  }
</style>
