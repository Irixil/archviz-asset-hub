<script lang="ts">
  import {
    assetApi,
    variantApi,
    mimeCategory,
    type Asset,
    type Variant,
    type CoveringWorkflow,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import AssetMetadata from '$lib/components/asset/AssetMetadata.svelte'
  import AssetTags from '$lib/components/asset/AssetTags.svelte'
  import AssetCollections from '$lib/components/asset/AssetCollections.svelte'
  import AssetProject from '$lib/components/asset/AssetProject.svelte'
  import AssetCustomFields from '$lib/components/asset/AssetCustomFields.svelte'
  import AssetMetadataPills from '$lib/components/asset/AssetMetadataPills.svelte'
  import SharedAsset from '$lib/components/SharedAsset.svelte'
  import MediaTagsTab from '$lib/components/MediaTagsTab.svelte'
  import TextTrackPanel from '$lib/components/text-tracks/TextTrackPanel.svelte'
  import AssetComments from '$lib/components/asset/AssetComments.svelte'
  import AssetActivity from '$lib/components/asset/AssetActivity.svelte'
  import LightboxPreviewPane from './LightboxPreviewPane.svelte'
  import LightboxPanelHeader from './LightboxPanelHeader.svelte'
  import LightboxTabBar, { type PanelTab } from './LightboxTabBar.svelte'
  import LightboxVariantsTab from './LightboxVariantsTab.svelte'
  import LightboxHistoryTab from './LightboxHistoryTab.svelte'
  import LightboxActionsTab from './LightboxActionsTab.svelte'
  import { type VariantTab } from '$lib/components/variants/VariantsTool.svelte'
  import { ALL_VARIANT_TOOLS } from '$lib/components/variants/toolDefs'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'
  import { useShortcuts } from '$lib/shortcuts'
  import {
    deriveVariantMime,
    isAudio as mimeIsAudio,
    isImage as mimeIsImage,
    isVideo as mimeIsVideo,
  } from '$lib/utils/mime'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { tick } from 'svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'

  /** /
  Lightbox (orchestrator, ~160 lines)
  ├── LightboxPreviewPane    ← selectedTool, showDraftOverlay, preview urls; bind:zoomIn/Out/Reset
  │   ├── SharedAsset / VariantDraftSession
  │   ├── VariantToolSidebar → onToolSelect → Lightbox relay → LightboxVariantsTab
  │   ├── VariantToolPanel   → oncreate → Lightbox relay → LightboxVariantsTab.handleCreate
  │   └── PreviewToolbar
  └── Panel div
      ├── LightboxPanelHeader   → onAssetRenamed → asset = updated
      ├── AssetMetadataPills
      ├── SharedAsset (mobile)
      ├── LightboxTabBar        → onTabChange → activeTab
      └── Tab content
          ├── [details]  inline (~15 lines)
          ├── [tags]     MediaTagsTab
          ├── [variants] LightboxVariantsTab  bind:selectedVariant/Tool/DraftOverlay/DraftAssetId/oncreate
          ├── [text]     TextTrackPanel
          ├── [comments] AssetComments
          ├── [history]  LightboxHistoryTab   → onVersionChanged → asset = updated
          ├── [activity] AssetActivity
          └── [actions]  LightboxActionsTab   → ondeleted, onOpenGridView
  /**/

  interface Props {
    asset: Asset | null
    onClose: () => void
    onDeleted: (id: string) => void
    onAssetUpdated?: (updated: Asset) => void
    onOpenGridView?: (assetId: string) => void
  }

  let {
    asset = $bindable(),
    onClose,
    onDeleted,
    onAssetUpdated,
    onOpenGridView,
  }: Props = $props()

  // --- Zoom refs for keyboard shortcuts ---
  let zoomIn = $state<(() => void) | undefined>(undefined)
  let zoomOut = $state<(() => void) | undefined>(undefined)
  let zoomReset = $state<(() => void) | undefined>(undefined)

  useShortcuts({
    'lightbox.zoom-in': () => zoomIn?.(),
    'lightbox.zoom-out': () => zoomOut?.(),
    'view.zoom-reset': () => {
      if (asset) zoomReset?.()
    },
  })

  // --- Panel tab routing ---
  let activeTab = $state<PanelTab>('details')
  const isMediaTagsAsset = $derived(
    asset ? mimeIsAudio(asset.mime_type) || mimeIsVideo(asset.mime_type) : false
  )

  // --- Variant relay state (written by LightboxVariantsTab, read by LightboxPreviewPane) ---
  let selectedVariant = $state<Variant | null>(null)
  let variants = $state<readonly Variant[]>([])
  let coveringWorkflow = $state<CoveringWorkflow | null>(null)
  let variantsLoading = $state(false)
  let selectedTool = $state<VariantTab | null>(null)
  let reuseParams = $state<Record<string, unknown> | null>(null)
  let reuseNonce = $state(0)
  let showDraftOverlay = $state(false)
  let variantsTabRef = $state<
    ReturnType<typeof LightboxVariantsTab> | undefined
  >(undefined)

  // --- Asset-level derived values ---
  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')
  const isImage = $derived(asset ? mimeIsImage(asset.mime_type) : false)
  const isVideo = $derived(asset ? mimeIsVideo(asset.mime_type) : false)
  const isAudio = $derived(asset ? mimeIsAudio(asset.mime_type) : false)

  const previewMimeType = $derived(
    selectedVariant
      ? deriveVariantMime(selectedVariant, asset?.mime_type ?? '')
      : (asset?.mime_type ?? '')
  )
  const previewCategory = $derived(mimeCategory(previewMimeType))
  const previewThumbUrl = $derived(
    asset
      ? selectedVariant
        ? variantApi.thumbUrl(asset.id, selectedVariant.id)
        : assetApi.thumbUrl(asset.id)
      : ''
  )
  const previewFileUrl = $derived(
    asset
      ? selectedVariant
        ? variantApi.fileUrl(asset.id, selectedVariant.id)
        : assetApi.fileUrl(asset.id)
      : ''
  )

  let assetDetail = $state<Asset | null>(null)

  async function loadAssetDetail() {
    if (!asset) return
    try {
      assetDetail = await assetApi.get(asset.id)
      loadVariants()
    } catch {
      // silently ignore — fall back to the prop
    }
  }

  async function loadVariants() {
    if (!asset) return
    variants = []
    variantsLoading = true
    try {
      const response = await variantApi.list(asset.id)
      variants = response?.variants ?? []
      coveringWorkflow = response?.covering_workflow ?? null
    } catch {
      toastStore.show(m.text_tracks_error_generic(), 'error')
    } finally {
      variantsLoading = false
    }
  }

  const activeProject = $derived(
    asset?.project_id
      ? (projectsStore.projects.find((p) => p.id === asset?.project_id) ?? null)
      : null
  )

  async function onOpenAsset(assetId: string) {
    const nextAsset =
      assetsStore.assets.find((candidate) => candidate.id === assetId) ??
      (await assetApi.get(assetId))
    asset = nextAsset
    onAssetUpdated?.(nextAsset)
  }

  function onDraftStarted(_nonce: string) {
    showDraftOverlay = true
  }

  function onDraftDone() {
    showDraftOverlay = false
    selectedTool = null
    variantsTabRef?.notifyDraftDone()
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (
      !asset ||
      event.key !== 'Escape' ||
      !viewportStore.isXl ||
      selectedTool === null
    )
      return
    selectedTool = null
  }

  function onToolSelect(tool: VariantTab | null) {
    selectedTool = tool
    reuseParams = null
    if (tool === null) showDraftOverlay = false
  }

  const KNOWN_TABS = new Set(ALL_VARIANT_TOOLS.map((t) => t.key))

  function applyReuseParams(params: Record<string, unknown> | null) {
    reuseParams = params
    reuseNonce += 1
  }

  function onReuse(variant: Variant) {
    if (!KNOWN_TABS.has(variant.type as VariantTab)) return
    let params: Record<string, unknown> | null = null
    if (variant.transform_params) {
      try {
        params = JSON.parse(variant.transform_params) as Record<string, unknown>
      } catch {
        params = null
      }
    }
    selectedTool = variant.type as VariantTab
    applyReuseParams(params)
  }

  async function handleCreate(type: string, params: object) {
    activeTab = 'variants'
    await tick()
    await variantsTabRef?.handleCreate(type, params)
  }

  function onAssetRenamed(updated: Asset) {
    asset = updated
    onAssetUpdated?.(updated)
  }

  function onVersionChanged(updated: Asset) {
    asset = updated
    onAssetUpdated?.(updated)
  }

  async function onThumbnailUpdated(updatedAsset?: Asset) {
    if (!asset) return
    const assetId = asset.id
    const updated = updatedAsset ?? (await assetApi.get(assetId))
    if (asset?.id !== assetId) return
    asset = updated
    assetDetail = updated
    assetsStore.patchAsset(assetId, updated)
    assetsStore.reloadAssetResources(assetId)
    onAssetUpdated?.(updated)
    loadVariants()
  }

  $effect(() => {
    if (!asset) {
      selectedVariant = null
      selectedTool = null
      showDraftOverlay = false
      assetDetail = null
      return
    }
    selectedVariant = null
    selectedTool = null
    variants = []
    loadAssetDetail()
  })
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if asset}
  <LightboxPreviewPane
    {asset}
    {previewMimeType}
    {previewCategory}
    {previewThumbUrl}
    {previewFileUrl}
    {selectedTool}
    {reuseParams}
    {reuseNonce}
    onReuseParams={applyReuseParams}
    {showDraftOverlay}
    draftAssetId={asset.id}
    creating={false}
    {onDraftStarted}
    {handleCreate}
    {onToolSelect}
    {onDraftDone}
    onVariantCommitted={loadVariants}
    {onClose}
    bind:zoomIn
    bind:zoomOut
    bind:zoomReset
  />

  <!-- Panel: fixed 25% -->
  <div
    transition:fly={{
      x: viewportStore.isMobile ? 0 : 420,
      y: viewportStore.isMobile ? 600 : 0,
      duration: 380,
      easing: cubicOut,
    }}
    class="asset-lightbox fixed inset-y-0 right-0 z-50 flex w-full flex-col bg-white shadow-2xl md:w-[25%] dark:bg-gray-900"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <LightboxPanelHeader {asset} {category} {onClose} {onAssetRenamed} />

    <div
      class="flex-shrink-0 border-b border-gray-100 px-5 py-3 dark:border-gray-800"
    >
      <AssetMetadataPills {asset} {category} {onThumbnailUpdated} />
    </div>

    <!-- Mobile-only inline preview -->
    <div
      class="relative z-0 block flex-shrink-0 overflow-hidden md:hidden {ASSET_BACKGROUND_COLORS[
        category
      ]}"
      style="height: 220px"
    >
      <div class="flex h-full items-center justify-center">
        <SharedAsset
          asset={{ ...asset, mime_type: previewMimeType }}
          category={previewCategory}
          thumbUrl={previewThumbUrl}
          assetUrl={previewFileUrl}
        />
      </div>
    </div>

    <LightboxTabBar
      {activeTab}
      {asset}
      {isMediaTagsAsset}
      onTabChange={(tab) => (activeTab = tab)}
    />

    <!-- Tab content (scrollable) -->
    <div class="flex-1 overflow-y-auto">
      {#if activeTab === 'details'}
        <div class="space-y-6 px-5 py-5">
          <AssetMetadata asset={assetDetail ?? asset} {onOpenAsset} />
          <AssetTags asset={assetDetail ?? asset} />
          <AssetCollections {asset} />
          {#if authStore.role !== 'viewer' || activeProject}
            <AssetProject {asset} {activeProject} />
          {/if}
          <AssetCustomFields {asset} />
        </div>
      {:else if activeTab === 'tags'}
        <MediaTagsTab assetId={asset.id} />
      {:else if activeTab === 'variants'}
        <LightboxVariantsTab
          bind:this={variantsTabRef}
          {asset}
          {variants}
          {isImage}
          {isVideo}
          {isAudio}
          {coveringWorkflow}
          {variantsLoading}
          bind:selectedTool
          bind:selectedVariant
          bind:showDraftOverlay
          {reuseParams}
          {reuseNonce}
          {onReuse}
          onReuseParams={applyReuseParams}
          {onThumbnailUpdated}
          onNavigate={(tab) => (activeTab = tab)}
          {onDraftStarted}
          {loadVariants}
        />
      {:else if activeTab === 'text'}
        <TextTrackPanel {asset} />
      {:else if activeTab === 'comments'}
        <AssetComments
          {asset}
          {variants}
          onVariantSelect={(v) => (selectedVariant = v)}
        />
      {:else if activeTab === 'history'}
        <LightboxHistoryTab {asset} {onVersionChanged} />
      {:else if activeTab === 'activity'}
        <AssetActivity {asset} />
      {:else if activeTab === 'actions'}
        <LightboxActionsTab
          {asset}
          {onDeleted}
          {onClose}
          {onOpenAsset}
          {onOpenGridView}
        />
      {/if}
    </div>
  </div>
{/if}
