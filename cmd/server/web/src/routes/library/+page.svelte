<script lang="ts">
  import { getContext, onMount } from 'svelte'
  import { page } from '$app/state'
  import { assetApi, type Asset, type MenuItem } from '$lib/api'
  import {
    APPSHELL_KEY,
    type AppShellContext,
  } from '$lib/components/AppShell.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { stackStore } from '$lib/stores/stack.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { createRubberBand } from '$lib/stores/rubberBand.svelte'
  import LibraryHeader from '$lib/components/LibraryHeader.svelte'
  import AssetGrid from '$lib/components/asset/AssetGrid.svelte'
  import ZoomOverlay from '$lib/components/ui/ZoomOverlay.svelte'
  import Lightbox from '$lib/components/lightbox/Lightbox.svelte'
  import AssetBulkActionBar from '$lib/components/asset/AssetBulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import ProjectInfoPanel from '$lib/components/ProjectInfoPanel.svelte'
  import ArchiveTaxonomyTabs from '$lib/components/ArchiveTaxonomyTabs.svelte'
  import CustomFieldFilters from '$lib/components/CustomFieldFilters.svelte'
  import SimilarityBanner from '$lib/components/SimilarityBanner.svelte'
  import UploadsTray from '$lib/components/UploadsTray.svelte'
  import { goto, pushState, replaceState } from '$app/navigation'
  import { Book, Box, LibraryBig, Menu, X } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { statusBarStore } from '$lib/stores/bottomStatusBar.svelte'
  import { useShortcuts, setShortcutContext } from '$lib/shortcuts'
  import GridModeButtons from '$lib/components/GridModeButtons.svelte'
  import SortButtons from '$lib/components/SortButtons.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { configStore } from '$lib/stores/config.svelte'

  let selectedAsset = $state<Asset | null>(null)
  let showPalette = $state(false)
  let showProjectShareModal = $state(false)
  let pendingDeleteIds = $state<string[]>([])
  let showDeleteConfirm = $state(false)
  const appShell = getContext<AppShellContext>(APPSHELL_KEY)

  const pendingDeleteNames = $derived(
    pendingDeleteIds.map(
      (id) =>
        assetsStore.assets.find((a) => a.id === id)?.original_filename ?? id
    )
  )
  let seenSplashScreen = $state(false)
  let isDraggingFiles = $state(false)
  let mainEl = $state<HTMLElement | undefined>(undefined)
  let zoomOverlay = $state<{ src: string; vars: string; asset: Asset } | null>(
    null
  )
  let draggingProjectCover = $state(false)
  let lastSimilarToParam = $state<string | null>(null)
  let lastSyncedUrl = $state('')

  let sort = $state<'mimetype' | 'created_at' | 'size' | 'taken_at'>(
    (assetsStore.sortKey as 'mimetype' | 'created_at' | 'size' | 'taken_at') ??
      'created_at'
  )
  let asc = $state(assetsStore.sortAsc)

  const activeProject = $derived(
    navigationStore.activeProjectId
      ? (projectsStore.projects.find(
          (p) => p.id === navigationStore.activeProjectId
        ) ?? null)
      : null
  )

  const activeCollection = $derived(
    navigationStore.activeCollectionId
      ? (collectionsStore.collections.find(
          (c) => c.id === navigationStore.activeCollectionId
        ) ?? null)
      : null
  )

  const projectShareTargets = $derived(
    activeProject
      ? [
          {
            type: 'project' as const,
            id: activeProject.id,
            label: activeProject.name,
            assetCount: activeProject.asset_count,
          },
        ]
      : []
  )

  const rb = createRubberBand(() => mainEl)

  function handleCardClick(asset: Asset, index: number, event: MouseEvent) {
    const isSimilaritySource =
      assetsStore.similarity?.anchor_asset_id === asset.id
    // In stack mode, shift-click selection is disabled — plain click falls through to lightbox
    const handled =
      !stackStore.active &&
      !isSimilaritySource &&
      selectionStore.handleCardClick(
        asset,
        index,
        assetsStore.assets,
        event,
        authStore.role !== 'viewer'
      )
    if (!handled) {
      const cardEl = (event.currentTarget as HTMLElement).closest(
        'button.asset-card'
      ) as HTMLElement | null
      const imgEl = cardEl?.querySelector('img') as HTMLImageElement | null
      const rect = cardEl?.getBoundingClientRect()

      if (rect && imgEl?.src) {
        cardEl?.classList.add('asset-card--opening')
        setTimeout(() => cardEl?.classList.remove('asset-card--opening'), 180)

        const ow = window.innerWidth * 0.7
        const oh = window.innerHeight * 0.7
        const sx = rect.width / ow
        const sy = rect.height / oh
        const cardCx = rect.left + rect.width / 2
        const cardCy = rect.top + rect.height / 2
        const tx = cardCx - window.innerWidth / 2
        const ty = cardCy - window.innerHeight / 2
        zoomOverlay = {
          src: imgEl.src,
          vars: `--tx:${tx}px; --ty:${ty}px; --sx:${sx}; --sy:${sy}`,
          asset,
        }
        // Match panel slide delay: panel starts at ~280ms, overlay fades by ~420ms
        setTimeout(() => {
          selectedAsset = asset
          zoomOverlay = null
        }, 280)
      } else {
        selectedAsset = asset
      }
    }
  }

  async function handleProjectSelect(item: MenuItem | null) {
    navigationStore.selectProject(item?.id ?? null)
    if (item?.id) await foldersStore.loadForProject(item.id)
    if (item?.url) goto(item.url)
  }

  function handleBulkDone() {
    selectionStore.clear()
    assetsStore.invalidate()
  }

  function libraryUrlWithSimilarity(assetId: string | null) {
    const url = new URL(page.url)
    if (assetId) {
      url.searchParams.set('similar_to', assetId)
    } else {
      url.searchParams.delete('similar_to')
    }
    return url.pathname + url.search
  }

  function syncSimilarityUrl(assetId: string | null, mode: 'push' | 'replace') {
    const next = libraryUrlWithSimilarity(assetId)
    if (next === lastSyncedUrl) return
    lastSyncedUrl = next
    if (mode === 'push') {
      pushState(next, {})
    } else {
      replaceState(next, {})
    }
  }

  function clearSimilarity() {
    assetsStore.clearSimilarity()
    syncSimilarityUrl(null, 'replace')
  }

  function navigateToSimilarity(assetId: string) {
    assetsStore.setSimilarTo(assetId, { load: false })
    syncSimilarityUrl(assetId, 'push')
    assetsStore.load(true)
  }

  function searchEntireWorkspace() {
    navigationStore.selectProject(null)
    assetsStore.setActiveTags([], { load: false })
    syncSimilarityUrl(assetsStore.similarTo, 'push')
    assetsStore.load(true)
  }

  async function confirmDeleteAssets() {
    const ids = pendingDeleteIds
    pendingDeleteIds = []
    await assetApi.bulkDelete(ids)
    handleBulkDone()
  }

  function handleDeleted(id: string) {
    assetsStore.remove(id)
    selectionStore.remove(id)
    selectedAsset = null
  }

  function handleMainDragOver(e: DragEvent) {
    if (!e.dataTransfer?.types.includes('Files')) return
    if (authStore.role === 'viewer') return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
    isDraggingFiles = true
  }

  function handleMainDragLeave(e: DragEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      isDraggingFiles = false
    }
  }

  function handleMainDrop(e: DragEvent) {
    isDraggingFiles = false
    if (!e.dataTransfer?.files.length) return
    if (authStore.role === 'viewer') return
    e.preventDefault()
    assetsStore.upload(
      Array.from(e.dataTransfer.files),
      activeProject?.id ?? null,
      navigationStore.activeFolderId ?? null
    )
  }

  function onDropProjectCover(e: DragEvent) {
    console.log('drop')
    e.preventDefault()
    draggingProjectCover = false
    const cover_asset_id = e.dataTransfer?.getData('text/plain')
    if (authStore.role === 'viewer') return
    if (!activeProject) return
    if (!cover_asset_id) return
    projectsStore.update(activeProject.id, { cover_asset_id })
  }
  function onDraggingProjectCover(e: DragEvent) {
    draggingProjectCover = true
    e.preventDefault()
  }
  function onDragLeaveProjectCover(e: DragEvent) {
    draggingProjectCover = false
    e.preventDefault()
  }

  const cols = $derived(
    1 + statusBarStore.maxZoom - Math.floor(statusBarStore.zoom)
  )

  $effect(() => {
    setShortcutContext(selectedAsset ? 'lightbox' : 'grid')
  })

  $effect(() => {
    const param = page.url.searchParams.get('similar_to')
    if (param === lastSimilarToParam) return
    lastSimilarToParam = param
    if (assetsStore.similarTo !== param) {
      assetsStore.setSimilarTo(param, { load: false })
    }
    lastSyncedUrl = page.url.pathname + page.url.search
  })

  function moveSelection(delta: number) {
    const assets = assetsStore.assets
    if (!assets.length) return
    const current = selectionStore.lastSelectedIndex
    const next = current < 0 ? 0 : current + delta
    selectionStore.moveSelectionTo(next, assets)
    const id = selectionStore.selectedIds.values().next().value
    document
      .querySelector<HTMLElement>(`[data-asset-id="${id}"]`)
      ?.scrollIntoView({ block: 'nearest' })
  }

  onMount(() => {
    seenSplashScreen =
      localStorage.getItem(`onboard_${authStore.workspace?.id}`) !== null
  })

  useShortcuts({
    'palette.open': () => {
      showPalette = !showPalette
    },
    'selection.all': () => selectionStore.selectAll(assetsStore.assets),
    'selection.clear': () => {
      selectionStore.clear()
      selectedAsset = null
    },
    'selection.invert': () =>
      selectionStore.invertSelection(assetsStore.assets),
    'asset.open-detail': () => {
      if (selectionStore.selectedIds.size === 1) {
        const id = selectionStore.selectedIds.values().next().value
        selectedAsset = assetsStore.assets.find((a) => a.id === id) ?? null
      }
    },
    'asset.delete': () => {
      if (selectionStore.selectedIds.size === 0) return
      pendingDeleteIds = [...selectionStore.selectedIds]
      showDeleteConfirm = true
    },
    'asset.download': () => {
      const asset =
        selectedAsset ??
        (selectionStore.selectedIds.size === 1
          ? (assetsStore.assets.find((a) =>
              selectionStore.selectedIds.has(a.id)
            ) ?? null)
          : null)
      if (!asset) return
      const a = document.createElement('a')
      a.href = assetApi.fileUrl(asset.id)
      a.download = asset.original_filename
      a.click()
    },
    'view.toggle-layout': () => {
      const modes = ['spaced', 'compact', 'table'] as const
      const next =
        modes[(modes.indexOf(statusBarStore.gridMode) + 1) % modes.length]
      statusBarStore.gridMode = next
    },
    'view.zoom-in': () => {
      statusBarStore.zoomIncrease()
    },
    'view.zoom-out': () => {
      statusBarStore.zoomDecrease()
    },
    'view.zoom-reset': () => {
      statusBarStore.zoomReset()
    },
    'lightbox.close': () => {
      selectedAsset = null
    },
    'lightbox.next': () => {
      if (!selectedAsset) return
      const assets = assetsStore.assets
      const idx = assets.findIndex((a) => a.id === selectedAsset!.id)
      if (idx < assets.length - 1) selectedAsset = assets[idx + 1]
    },
    'lightbox.prev': () => {
      if (!selectedAsset) return
      const assets = assetsStore.assets
      const idx = assets.findIndex((a) => a.id === selectedAsset!.id)
      if (idx > 0) selectedAsset = assets[idx - 1]
    },
    'lightbox.download': () => {
      if (!selectedAsset) return
      const a = document.createElement('a')
      a.href = assetApi.fileUrl(selectedAsset.id)
      a.download = selectedAsset.original_filename
      a.click()
    },
    'selection.move-right': () => moveSelection(+1),
    'selection.move-left': () => moveSelection(-1),
    'selection.move-up': () => moveSelection(-cols),
    'selection.move-down': () => moveSelection(+cols),
  })
</script>

<svelte:head>
  <title>{m.library()}｜建筑素材中台</title>
</svelte:head>

<svelte:window
  onmousemove={(e) => rb.onMouseMove(e)}
  onmouseup={() => rb.onMouseUp((ids) => selectionStore.selectByIds(ids))}
/>

<LibraryHeader
  showShareButton={!!activeProject}
  onShareProject={() => {
    showProjectShareModal = true
  }}
>
  {#snippet prefix()}
    <button
      type="button"
      class="flex items-center gap-2 rounded-xl p-1 transition-colors hover:bg-[var(--bg-hover)]"
      onclick={() => {
        if (viewportStore.isMobile) {
          appShell.openDrawer()
          return
        }
        navigationStore.toggleSidebarVisible()
      }}
      aria-label={m.menu()}
    >
      {#if viewportStore.isMobile}
        <span
          class="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--bg-elevated)] text-[var(--text-secondary)]"
        >
          <Menu class="h-5 w-5" />
        </span>
      {/if}
      {#if activeProject}
        <div
          role="img"
          class={`rounded border-1 transition-colors ${draggingProjectCover ? 'border-green-500' : 'border-transparent'} `}
          ondragleave={onDragLeaveProjectCover}
          ondragover={onDraggingProjectCover}
          ondrop={onDropProjectCover}
        >
          {#if activeProject?.cover_asset_id}
            <img
              src={assetApi.thumbUrl(activeProject.cover_asset_id)}
              class="h-10 w-10 rounded object-cover"
              alt="项目封面"
            />
          {:else}
            <Box class="h-10 w-10 rounded text-[var(--text-secondary)]" />
          {/if}
        </div>
      {:else if activeCollection}
        <Book class="h-10 w-10 rounded text-[var(--text-secondary)]" />
      {:else}
        <LibraryBig class="h-10 w-10 rounded text-[var(--text-secondary)]" />
      {/if}
    </button>
  {/snippet}
</LibraryHeader>

{#if activeProject}
  <ProjectInfoPanel project={activeProject} />
{/if}

{#if activeCollection}
  <div
    class="flex flex-wrap items-center gap-2 border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 sm:px-6"
  >
    <LibraryBig class="h-4 w-4 text-[var(--accent)]" />
    <span class="text-sm font-medium text-[var(--accent-text)]"
      >{activeCollection.name}</span
    >
    <span class="text-xs text-[var(--text-muted)]"
      >{m.assets_count({ count: activeCollection.asset_count })}</span
    >
    <button
      type="button"
      onclick={() => {
        navigationStore.selectCollection(null)
        assetsStore.invalidate()
      }}
      class="ml-1 rounded p-0.5 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
      aria-label="清除合集筛选"
    >
      <X class="h-3.5 w-3.5" />
    </button>
  </div>
{/if}

<ArchiveTaxonomyTabs
  activeTags={assetsStore.activeTags}
  onchange={(tags) => assetsStore.setActiveTags(tags)}
/>

{#if assetsStore.similarTo && assetsStore.similarity}
  <SimilarityBanner
    anchorFilename={assetsStore.similarity.anchor_filename}
    anchorAssetId={assetsStore.similarity.anchor_asset_id}
    resultCount={assetsStore.similarity.result_count}
    onClear={clearSimilarity}
    onOpenAnchor={async () => {
      if (!assetsStore.similarity) return
      selectedAsset = await assetApi.get(assetsStore.similarity.anchor_asset_id)
    }}
  />
{/if}

<div
  class="flex min-h-12 items-center justify-between gap-3 overflow-x-auto border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 sm:px-4"
>
  <div class="flex shrink-0 items-baseline gap-2">
    <span class="text-xs font-semibold text-[var(--text-primary)]"
      >当前结果</span
    >
    <span class="text-xs text-[var(--text-muted)] tabular-nums"
      >{assetsStore.assets.length} 项</span
    >
  </div>
  <div class="flex shrink-0 items-center gap-2">
    <SortButtons
      sort={(key, a) => assetsStore.sort(key, a)}
      bind:value={sort}
      bind:asc
      keys={{
        created_at: m.sort_date(),
        mimetype: m.sort_mimetype(),
        size: m.sort_weight(),
        ...(configStore.state.exif_keep
          ? { taken_at: m.sort_date_taken() }
          : {}),
      }}
    />
    <div class="h-4 w-px bg-[var(--border-default)]"></div>
    <GridModeButtons
      mode={statusBarStore.gridMode}
      onchange={(m) => {
        statusBarStore.gridMode = m
      }}
    />
  </div>
</div>

<CustomFieldFilters
  activeFilters={assetsStore.fieldFilters}
  onchange={(filters) => assetsStore.setFieldFilters(filters)}
/>

{#if zoomOverlay}
  <ZoomOverlay
    src={zoomOverlay.src}
    vars={zoomOverlay.vars}
    asset={zoomOverlay.asset}
  />
{/if}

<AssetGrid
  bind:mainEl
  {sort}
  gridMode={statusBarStore.gridMode}
  {isDraggingFiles}
  emptyProjectName={activeProject?.name ?? null}
  {seenSplashScreen}
  onCardClick={handleCardClick}
  onDismissOnboarding={() => {
    seenSplashScreen = true
    localStorage.setItem(`onboard_${authStore.workspace?.id}`, 'true')
  }}
  onDragOver={handleMainDragOver}
  onDragLeave={handleMainDragLeave}
  onDrop={handleMainDrop}
  onMouseDown={(e) => rb.onMouseDown(e)}
  onSearchEntireWorkspace={searchEntireWorkspace}
/>

{#if rb.band && rb.band.w > 2 && rb.band.h > 2}
  <div
    class="pointer-events-none fixed z-30 rounded-sm border border-[var(--accent)] bg-[var(--accent-soft)]"
    style="left:{rb.band.x}px; top:{rb.band.y}px; width:{rb.band
      .w}px; height:{rb.band.h}px"
  ></div>
{/if}

<Lightbox
  asset={selectedAsset}
  onClose={() => (selectedAsset = null)}
  onDeleted={handleDeleted}
  onAssetUpdated={(updated) => {
    selectedAsset = updated
    assetsStore.patchAsset(updated.id, updated)
    assetsStore.reloadAssetResources(updated.id)
  }}
  onOpenGridView={(id) => {
    selectedAsset = null
    navigateToSimilarity(id)
  }}
/>

<AssetBulkActionBar
  selectedIds={selectionStore.selectedIds}
  projects={projectsStore.projects}
  ondone={handleBulkDone}
  onclear={() => selectionStore.clear()}
/>

<UploadsTray />

{#if showPalette}
  <CommandPalette
    projects={projectsStore.projects}
    onselect={(item) => {
      handleProjectSelect(item)
      showPalette = false
    }}
    onclose={() => {
      showPalette = false
    }}
  />
{/if}

{#if showProjectShareModal && projectShareTargets.length > 0}
  <ShareModal
    bind:open={showProjectShareModal}
    targets={projectShareTargets}
    onclose={() => {
      showProjectShareModal = false
    }}
  />
{/if}

<ConfirmModal
  bind:open={showDeleteConfirm}
  title={m.delete_n_assets({ count: pendingDeleteIds.length })}
  items={pendingDeleteNames}
  onConfirm={confirmDeleteAssets}
  onCancel={() => {
    showDeleteConfirm = false
    pendingDeleteIds = []
  }}
/>

<style>
  .tags-scroll {
    scrollbar-width: none;
  }
  .tags-scroll::-webkit-scrollbar {
    display: none;
  }
</style>
