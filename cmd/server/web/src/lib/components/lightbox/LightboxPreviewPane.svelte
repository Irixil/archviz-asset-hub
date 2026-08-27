<script lang="ts">
  import { type Asset } from '$lib/api'
  import { ALL_VARIANT_TOOLS } from '$lib/components/variants/toolDefs'
  import { type VariantTab } from '$lib/components/variants/VariantsTool.svelte'
  import SharedAsset from '$lib/components/SharedAsset.svelte'
  import PreviewToolbar from '$lib/components/ui/PreviewToolbar.svelte'
  import VariantToolSidebar from '$lib/components/variants/VariantToolSidebar.svelte'
  import VariantToolPanel from '$lib/components/variants/VariantToolPanel.svelte'
  import VariantDraftSession from '$lib/components/variants/VariantDraftSession.svelte'
  import Backdrop from '$lib/components/ui/Backdrop.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { goto } from '$app/navigation'
  import { m } from '$lib/paraglide/messages'
  import { onMount } from 'svelte'

  interface Props {
    asset: Asset
    previewMimeType: string
    previewCategory: 'image' | 'video' | 'audio' | 'document'
    previewThumbUrl: string
    previewFileUrl: string
    selectedTool: VariantTab | null
    reuseParams?: Record<string, unknown> | null
    reuseNonce?: number
    onReuseParams?: (params: Record<string, unknown>) => void
    showDraftOverlay: boolean
    draftAssetId: string | null
    creating: boolean
    onClose: () => void
    onToolSelect: (tool: VariantTab | null) => void
    handleCreate: (type: string, params: object) => Promise<void>
    onDraftStarted: (nonce: string) => void
    onDraftDone: () => void
    onVariantCommitted: () => void
    // Zoom refs exposed upward for keyboard shortcut wiring
    zoomIn?: (() => void) | undefined
    zoomOut?: (() => void) | undefined
    zoomReset?: (() => void) | undefined
  }

  let {
    asset,
    previewMimeType,
    previewCategory,
    previewThumbUrl,
    previewFileUrl,
    selectedTool,
    reuseParams = null,
    reuseNonce = 0,
    onReuseParams,
    showDraftOverlay,
    draftAssetId,
    creating,
    onClose,
    onToolSelect,
    handleCreate,
    onDraftStarted,
    onDraftDone,
    onVariantCommitted,
    zoomIn = $bindable(undefined),
    zoomOut = $bindable(undefined),
    zoomReset = $bindable(undefined),
  }: Props = $props()

  let mounted = $state(false)
  onMount(() => {
    mounted = true
  })

  let previewContainer = $state<HTMLElement | null>(null)
  let showToolbar = $state<(() => void) | undefined>(undefined)
  let zoomWheel = $state<((e: WheelEvent) => void) | undefined>(undefined)
  let rotateRight = $state<(() => void) | undefined>(undefined)

  // Draft overlay state — lives here because VariantDraftSession is rendered here
  let draftSessionRef = $state<
    ReturnType<typeof VariantDraftSession> | undefined
  >(undefined)
  let draftDoneTimer: ReturnType<typeof setTimeout> | null = null

  const visibleVariantTools = $derived.by(() => {
    const mimeType = asset.mime_type
    return ALL_VARIANT_TOOLS.filter(
      (tool) =>
        tool.showFor(mimeType) &&
        !(tool.key === 'trigger_workflow' && authStore.role === 'viewer')
    )
  })

  function handleDraftStarted(nonce: string, meta?: Record<string, unknown>) {
    onDraftStarted(nonce)
    Promise.resolve().then(() => draftSessionRef?.addDraft(nonce, meta))
  }

  async function handleFfmpegKeep(
    _nonce: string,
    meta: Record<string, unknown> | undefined
  ): Promise<void> {
    const command = (meta?.command as string | undefined) ?? ''
    await handleCreate('custom_ffmpeg', { command })
  }

  function handleDraftDone() {
    if (draftDoneTimer) clearTimeout(draftDoneTimer)
    draftDoneTimer = setTimeout(() => {
      draftDoneTimer = null
      onDraftDone()
    }, 1200)
  }

  function handleClose(e: MouseEvent) {
    if (showDraftOverlay) return
    const src = e.target as HTMLElement
    if (src.classList.contains('asset-preview-full')) return
    if (src.classList.contains('asset-preview-toolbar')) return
    onClose()
  }
</script>

<Backdrop class="asset-lightbox-bg w-screen" {onClose}>
  <div
    bind:this={previewContainer}
    class="asset-preview-container fixed inset-y-0 left-0 hidden w-[75%] flex-col md:flex"
    role="button"
    tabindex="-1"
    onclick={handleClose}
    onkeydown={(e) => e.key === 'Enter' && onClose()}
    onwheel={(e) => zoomWheel?.(e)}
    onmousemove={() => showToolbar?.()}
    aria-label={m.close()}
  >
    <div class="relative grid min-h-0 flex-1 place-items-center p-40">
      {#if showDraftOverlay && draftAssetId}
        <div
          class="pointer-events-auto absolute inset-y-0 right-0 overflow-y-auto p-8"
          style="left: calc(60px + 350px);"
        >
          <div class="flex min-h-full flex-col items-center justify-center">
            <div class="w-full" style="max-width: 700px;">
              <VariantDraftSession
                bind:this={draftSessionRef}
                assetId={draftAssetId}
                gridMode={true}
                detectMediaKind={selectedTool === 'custom_ffmpeg'}
                onKeepDraft={selectedTool === 'custom_ffmpeg'
                  ? handleFfmpegKeep
                  : undefined}
                onDone={handleDraftDone}
                onAddMore={() => {}}
                {onVariantCommitted}
              />
            </div>
          </div>
        </div>
      {:else}
        <SharedAsset
          asset={{ ...asset, mime_type: previewMimeType }}
          category={previewCategory}
          thumbUrl={previewThumbUrl}
          assetUrl={previewFileUrl}
          bind:zoomIn
          bind:zoomOut
          bind:zoomReset
          bind:onwheel={zoomWheel}
          bind:rotateRight
        />
      {/if}

      {#if mounted && viewportStore.isXl && visibleVariantTools.length > 0}
        <VariantToolSidebar
          {asset}
          activeTool={selectedTool}
          {creating}
          onSelect={(tool) => {
            onToolSelect(tool)
          }}
        />
      {/if}

      {#if mounted && viewportStore.isXl && selectedTool !== null}
        <VariantToolPanel
          tool={selectedTool}
          {asset}
          {creating}
          {reuseParams}
          {reuseNonce}
          {onReuseParams}
          {handleCreate}
          onClose={() => {
            onToolSelect(null)
          }}
          onDraftStarted={handleDraftStarted}
          sessionActive={showDraftOverlay}
          onApplied={(results) => {
            onToolSelect(null)
            for (const { workflowName, runIds } of results) {
              toastStore.show(
                m.workflows_library_run_started({
                  name: workflowName,
                  count: runIds.length,
                }),
                'success',
                {
                  label: m.workflows_library_view_runs(),
                  onClick: () => void goto(`/library/settings/workflows/runs`),
                }
              )
            }
          }}
        />
      {/if}

      {#if !showDraftOverlay}
        <div
          class="pointer-events-auto absolute bottom-8 left-1/2 z-30 -translate-x-1/2"
        >
          <PreviewToolbar
            {zoomIn}
            {zoomOut}
            {rotateRight}
            fullscreenTarget={previewContainer}
            bind:show={showToolbar}
          />
        </div>
      {/if}
    </div>
  </div>
</Backdrop>
