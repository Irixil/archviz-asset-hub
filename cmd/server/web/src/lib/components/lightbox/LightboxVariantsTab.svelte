<script lang="ts">
  import {
    variantApi,
    type Asset,
    type CoveringWorkflow,
    type Variant,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { sseEvents } from '$lib/stores/assets.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { Inbox, Zap } from '@lucide/svelte'
  import Pills from '$lib/components/ui/Pills.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import ProgressBar from '$lib/components/ui/ProgressBar.svelte'
  import AssetVariantsGrid from '$lib/components/asset/AssetVariantsGrid.svelte'
  import VariantsTool, {
    type VariantTab,
  } from '$lib/components/variants/VariantsTool.svelte'
  import ParamHistoryButton from '$lib/components/variants/ParamHistoryButton.svelte'
  import { formatParamSummary } from '$lib/components/variants/paramHistorySummary'
  import { PARAM_HISTORY_TOOLS } from '$lib/components/variants/toolDefs'
  import VariantSharingHeader from '$lib/components/variants/VariantSharingHeader.svelte'
  import VariantPromoteForm from '$lib/components/variants/VariantPromoteForm.svelte'
  import CoveringWorkflowBanner from '$lib/components/variants/CoveringWorkflowBanner.svelte'
  import CreateVariantAutomationModal from '$lib/components/variants/CreateVariantAutomationModal.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { m } from '$lib/paraglide/messages'
  import type { PanelTab } from './LightboxTabBar.svelte'

  interface Props {
    asset: Asset
    variants: readonly Variant[]
    coveringWorkflow: CoveringWorkflow | null
    variantsLoading: boolean
    isImage: boolean
    isVideo: boolean
    isAudio: boolean
    loadVariants: () => void
    onThumbnailUpdated: () => void
    onNavigate: (tab: PanelTab) => void
    onDraftStarted: (nonce: string) => void
    // Bindable state relayed to sibling LightboxPreviewPane via Lightbox
    selectedVariant?: Variant | null
    selectedTool?: VariantTab | null
    showDraftOverlay?: boolean
    reuseParams?: Record<string, unknown> | null
    reuseNonce?: number
    onReuse: (variant: Variant) => void
    onReuseParams?: (params: Record<string, unknown>) => void
  }

  let {
    asset,
    variants,
    coveringWorkflow,
    variantsLoading,
    isImage,
    isVideo,
    isAudio,
    onThumbnailUpdated,
    onNavigate,
    loadVariants,
    onDraftStarted: _onDraftStarted,
    selectedVariant = $bindable<Variant | null>(null),
    selectedTool = $bindable<VariantTab | null>(null),
    showDraftOverlay = $bindable(false),
    reuseParams = null,
    reuseNonce = 0,
    onReuse,
    onReuseParams,
  }: Props = $props()

  // --- Variant list state ---
  let showAutomationModal = $state(false)
  let creating = $state(false)
  let pendingVariantAssetId = $state<string | null>(null)
  let variantRefreshProgress = $state(0)
  let createError = $state('')
  let createSuccess = $state('')
  let variantRefreshTimer: ReturnType<typeof setInterval> | null = null
  let variantRefreshTimeout: ReturnType<typeof setTimeout> | null = null
  let variantPanelState = $state<
    { mode: 'list' } | { mode: 'promote'; variant: Variant }
  >({ mode: 'list' })
  let activeVariantTab = $state<VariantTab>('all')

  const automatableVariants = $derived(
    variants.filter((variant) => variant.type !== 'manual')
  )
  const canCreateVariantAutomation = $derived(
    authStore.role === 'owner' &&
      automatableVariants.length > 0 &&
      !coveringWorkflow
  )
  const variantSubTabs = $derived([
    { id: 'all' as VariantTab, label: m.all() },
    ...(isImage
      ? [
          { id: 'image_resize' as VariantTab, label: m.resize() },
          { id: 'image_watermark' as VariantTab, label: m.watermark() },
          { id: 'image_convert' as VariantTab, label: m.convert() },
          { id: 'image_smart_crop' as VariantTab, label: m.smart_crop() },
          { id: 'image_crop' as VariantTab, label: m.crop() },
          { id: 'image_bg_remove' as VariantTab, label: m.bg_remove() },
          { id: 'image_with_prompt' as VariantTab, label: 'AI Transform' },
        ]
      : []),
    ...(isVideo
      ? [
          { id: 'video_transcode' as VariantTab, label: m.transcode() },
          { id: 'video_watermark' as VariantTab, label: m.watermark() },
          { id: 'video_capture_image' as VariantTab, label: m.thumbnail() },
          { id: 'video_extract' as VariantTab, label: m.audio_extract_tab() },
        ]
      : []),
    ...(isAudio
      ? [
          {
            id: 'audio_transcode' as VariantTab,
            label: m.audio_transcode_tab(),
          },
          {
            id: 'audio_normalize' as VariantTab,
            label: m.audio_normalize_tab(),
          },
        ]
      : []),
    {
      id: 'custom_ffmpeg' as VariantTab,
      label: m.variant_tool_custom_ffmpeg(),
    },
  ])

  // Reset state when asset changes
  $effect(() => {
    void asset.id
    variants = []
    coveringWorkflow = null
    showAutomationModal = false
    variantPanelState = { mode: 'list' }
    selectedVariant = null
    selectedTool = null
    loadVariants()
  })

  // Hide draft overlay when tool is deselected
  $effect(() => {
    if (selectedTool === null) {
      showDraftOverlay = false
    }
  })

  // SSE: variant ready
  $effect(() => {
    const ev = sseEvents.last
    if (pendingVariantAssetId !== asset.id) return
    if (ev?.type !== 'variant_ready' || ev.payload.asset_id !== asset.id) return
    pendingVariantAssetId = null
    activeVariantTab = 'all'
    selectedTool = null
    createSuccess = 'Almost ready. Refreshing shortly…'
    scheduleVariantRefresh(asset.id)
  })

  // SSE: variant failed
  $effect(() => {
    const ev = sseEvents.last
    if (pendingVariantAssetId !== asset.id) return
    if (ev?.type !== 'variant_failed' || ev.payload.asset_id !== asset.id)
      return
    clearVariantRefreshCountdown()
    pendingVariantAssetId = null
    createSuccess = ''
    createError = ev.payload.error || m.variant_create_failed()
  })

  // Cleanup on unmount
  $effect(() => {
    return () => {
      clearVariantRefreshCountdown()
      activeVariantTab = 'all'
      createSuccess = ''
      createError = ''
    }
  })

  function replaceVariant(updatedVariant: Variant) {
    variants = variants.map((v) =>
      v.id === updatedVariant.id ? updatedVariant : v
    )
    if (selectedVariant?.id === updatedVariant.id)
      selectedVariant = updatedVariant
  }

  function replaceVariants(nextVariants: readonly Variant[]) {
    variants = nextVariants
    if (selectedVariant) {
      selectedVariant =
        nextVariants.find((v) => v.id === selectedVariant?.id) ?? null
    }
  }

  function clearVariantRefreshCountdown() {
    if (variantRefreshTimer) {
      clearInterval(variantRefreshTimer)
      variantRefreshTimer = null
    }
    if (variantRefreshTimeout) {
      clearTimeout(variantRefreshTimeout)
      variantRefreshTimeout = null
    }
    variantRefreshProgress = 0
  }

  function scheduleVariantRefresh(assetId: string) {
    clearVariantRefreshCountdown()
    variantRefreshProgress = 100
    const startedAt = Date.now()
    const durationMs = 10000
    variantRefreshTimer = setInterval(() => {
      const elapsed = Date.now() - startedAt
      variantRefreshProgress = Math.max(0, 1 - elapsed / durationMs) * 100
    }, 100)
    variantRefreshTimeout = setTimeout(async () => {
      clearVariantRefreshCountdown()
      if (asset.id !== assetId) return
      await loadVariants()
    }, durationMs)
  }

  export async function handleCreate(type: string, params: object) {
    creating = true
    createError = ''
    createSuccess = ''
    try {
      const result = await variantApi.create(asset.id, type, params)
      pendingVariantAssetId = asset.id
      createSuccess = `Queued (job ${result.job_id.slice(0, 8)}). Waiting for completion…`
      toastStore.show(createSuccess, 'success')
      onNavigate('variants')
    } catch (e: unknown) {
      pendingVariantAssetId = null
      createError = e instanceof Error ? e.message : m.variant_create_failed()
      toastStore.show(createError, 'error')
    } finally {
      creating = false
    }
  }

  async function handleDeleteVariant(variantId: string) {
    try {
      await variantApi.delete(asset.id, variantId)
      variants = variants.filter((v) => v.id !== variantId)
    } catch {
      /* silently ignore */
    }
  }

  function openPromoteVariant(variant: Variant) {
    variantPanelState = { mode: 'promote', variant }
  }

  function handleReuse(variant: Variant) {
    onReuse(variant)
    activeVariantTab = variant.type as VariantTab
  }

  export function notifyDraftDone() {
    showDraftOverlay = false
    selectedTool = null
    loadVariants()
  }
</script>

<div class="flex flex-col">
  {#if !viewportStore.isXl}
    <Pills
      pills={variantSubTabs}
      active={activeVariantTab}
      set={(p) => {
        activeVariantTab = p.id as VariantTab
        createError = ''
        createSuccess = ''
      }}
    />
  {/if}
  <Feedback error={createError} success={createSuccess} />

  <div class="px-5 py-4">
    {#if (viewportStore.isXl || activeVariantTab === 'all') && variantPanelState.mode === 'promote'}
      <VariantPromoteForm
        assetId={asset.id}
        assetFilename={asset.original_filename}
        variant={variantPanelState.variant}
        onCancel={() => {
          variantPanelState = { mode: 'list' }
        }}
        onSuccess={async () => {
          variantPanelState = { mode: 'list' }
          await loadVariants()
        }}
      />
    {:else if viewportStore.isXl || activeVariantTab === 'all'}
      {#if coveringWorkflow}
        <CoveringWorkflowBanner workflow={coveringWorkflow} />
      {/if}
      <VariantSharingHeader
        assetId={asset.id}
        {variants}
        onUpdate={replaceVariants}
      />
      {#if canCreateVariantAutomation}
        <button
          type="button"
          class="mb-4 flex w-full items-center justify-center gap-2 rounded-lg border border-emerald-200 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-50 dark:border-emerald-900/60 dark:text-emerald-300 dark:hover:bg-emerald-950/30"
          onclick={() => {
            showAutomationModal = true
          }}
        >
          <Zap class="h-4 w-4" />
          {m.variant_automation_button()}
        </button>
      {/if}
      {#if variantRefreshProgress > 0}
        <div class="mb-4 space-y-2">
          <p
            class="text-xs font-medium tracking-[0.08em] text-[var(--text-muted)] uppercase"
          >
            Refreshing variants in 10 seconds
          </p>
          <ProgressBar value={variantRefreshProgress} />
        </div>
      {/if}
      {#if variantsLoading}
        <div class="flex justify-center py-12">
          <Spinner size="md" />
        </div>
      {:else if variants.length === 0}
        <div class="flex flex-col items-center gap-4 py-10 text-center">
          <div
            class="flex h-14 w-14 items-center justify-center rounded-2xl bg-indigo-50 text-indigo-400 dark:bg-indigo-900/25 dark:text-indigo-400"
          >
            <Inbox class="h-7 w-7" />
          </div>
          <div>
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.no_variants_yet()}
            </p>
            {#if authStore.role !== 'viewer' && (isImage || isVideo)}
              <p class="mt-1 text-sm text-[var(--text-muted)]">
                {viewportStore.isXl
                  ? m.variant_use_tool_bar()
                  : m.variant_use_tabs()}
              </p>
            {/if}
          </div>
        </div>
      {:else}
        <AssetVariantsGrid
          {asset}
          {variants}
          {selectedVariant}
          onSelectVariant={(v) => {
            selectedVariant = selectedVariant?.id === v.id ? null : v
          }}
          onVariantUpdated={replaceVariant}
          onVariantsUpdated={replaceVariants}
          deleteVariant={handleDeleteVariant}
          promoteVariant={openPromoteVariant}
          thumbnailUpdated={onThumbnailUpdated}
          rerunVariant={loadVariants}
          reuseVariant={handleReuse}
        />
      {/if}
    {:else}
      {#if onReuseParams && PARAM_HISTORY_TOOLS.has(activeVariantTab)}
        <div class="mb-2 flex justify-end">
          <ParamHistoryButton
            variantType={activeVariantTab}
            disabled={creating}
            onSelect={onReuseParams}
            formatEntry={(params) =>
              formatParamSummary(activeVariantTab, params)}
            dropdownWidthClass={activeVariantTab === 'custom_ffmpeg'
              ? 'w-96'
              : 'w-64'}
            monospaceEntries={activeVariantTab === 'custom_ffmpeg'}
          />
        </div>
      {/if}
      {#key `${activeVariantTab}-${reuseNonce}`}
        <VariantsTool
          {asset}
          creating={creating || pendingVariantAssetId === asset.id}
          tool={activeVariantTab}
          initialParams={reuseParams}
          {handleCreate}
          onDone={async () => {
            activeVariantTab = 'all'
            await loadVariants()
          }}
        />
      {/key}
    {/if}
  </div>
</div>

{#if showAutomationModal}
  <CreateVariantAutomationModal
    assetId={asset.id}
    assetProjectId={asset.project_id ?? null}
    assetFolderId={asset.folder_id ?? null}
    assetVariants={automatableVariants}
    onClose={() => {
      showAutomationModal = false
    }}
  />
{/if}
