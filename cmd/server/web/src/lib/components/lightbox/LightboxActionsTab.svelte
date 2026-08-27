<script lang="ts">
  import { onDestroy } from 'svelte'
  import { assetApi, type Asset, type VisualSimilarResult } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { Images, Loader2, RefreshCw, Share } from '@lucide/svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import AssetExportImage from '$lib/components/asset/AssetExportImage.svelte'
  import AssetDeleteButton from '$lib/components/asset/AssetDeleteButton.svelte'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import PublicUrlMenuItem from '$lib/components/asset/PublicUrlMenuItem.svelte'
  import VisualSimilarityDrawer from './VisualSimilarityDrawer.svelte'
  import { m } from '$lib/paraglide/messages'
  import { isImage } from '$lib/utils/mime'

  interface Props {
    asset: Asset
    onClose: () => void
    onDeleted: (id: string) => void
    onOpenAsset?: (assetId: string) => void
    onOpenGridView?: (assetId: string) => void
  }

  let { asset, onDeleted, onClose, onOpenAsset, onOpenGridView }: Props =
    $props()

  let regenThumbLoading = $state(false)
  let showShareModal = $state(false)

  // Visual similarity state
  let similarLoading = $state(false)
  let similarResults = $state<VisualSimilarResult[] | null>(null)
  let showSimilarDrawer = $state(false)
  let noResultsMsg = $state(false)
  let noResultsMsgTimer: ReturnType<typeof setTimeout> | undefined

  onDestroy(() => clearTimeout(noResultsMsgTimer))

  const isImg = $derived(isImage(asset.mime_type))
  const shareTargets = $derived([
    { type: 'asset' as const, id: asset.id, label: m.selected_asset() },
  ])

  async function handleRegenerateThumbnail() {
    regenThumbLoading = true
    try {
      await assetApi.regenerateThumbnail(asset.id)
    } catch {
      /* silently ignore */
    } finally {
      regenThumbLoading = false
    }
  }

  async function handleFindSimilar() {
    similarLoading = true
    noResultsMsg = false
    try {
      const res = await assetApi.findSimilar(asset.id)
      similarResults = res.results
      if (res.results.length === 0) {
        noResultsMsg = true
        clearTimeout(noResultsMsgTimer)
        noResultsMsgTimer = setTimeout(() => {
          noResultsMsg = false
        }, 3000)
      } else {
        showSimilarDrawer = true
      }
    } catch {
      /* silently ignore */
    } finally {
      similarLoading = false
    }
  }

  function handleSimilarSelect(result: VisualSimilarResult) {
    showSimilarDrawer = false
    onOpenAsset?.(result.asset_id)
  }
</script>

{#if showSimilarDrawer && similarResults}
  <div class="h-full px-0 py-0">
    <VisualSimilarityDrawer
      anchorAssetId={asset.id}
      results={similarResults}
      onBack={() => {
        showSimilarDrawer = false
      }}
      onSelect={handleSimilarSelect}
      onOpenGridView={(id) => {
        onClose?.()
        onOpenGridView?.(id)
      }}
    />
  </div>
{:else}
  <div class="space-y-6 px-5 py-5">
    <div>
      <SubSectionTitle>{m.quick_actions()}</SubSectionTitle>
      <div class="space-y-2">
        <button
          class="flex w-full items-center gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-sm font-medium whitespace-nowrap text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)]"
          onclick={() => {
            showShareModal = true
          }}
        >
          <Share class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
          {m.share()}
        </button>

        <PublicUrlMenuItem assetId={asset.id} />

        {#if isImg}
          {#if noResultsMsg}
            <p class="px-4 py-3 text-sm text-[var(--text-muted)]">
              {m.no_similar_results()}
            </p>
          {:else}
            <button
              class="flex w-full items-center gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-sm font-medium whitespace-nowrap text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)] disabled:opacity-50"
              onclick={handleFindSimilar}
              disabled={similarLoading}
            >
              {#if similarLoading}
                <Loader2
                  class="h-4 w-4 shrink-0 animate-spin text-[var(--text-muted)]"
                />
              {:else}
                <Images class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
              {/if}
              {similarLoading ? m.loading() : m.find_similar()}
            </button>
          {/if}
        {/if}

        {#if authStore.role !== 'viewer'}
          <button
            class="flex w-full items-center gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-sm font-medium whitespace-nowrap text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)] disabled:opacity-50"
            onclick={handleRegenerateThumbnail}
            disabled={regenThumbLoading}
          >
            <RefreshCw class="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
            {regenThumbLoading ? m.queuing_() : m.thumbnail_regen()}
          </button>
        {/if}
      </div>
    </div>

    <AssetExportImage {asset} />

    {#if authStore.role !== 'viewer'}
      <AssetDeleteButton {asset} ondeleted={onDeleted} />
    {/if}
  </div>
{/if}

{#if showShareModal}
  <ShareModal
    bind:open={showShareModal}
    targets={shareTargets}
    onclose={() => {
      showShareModal = false
    }}
  />
{/if}
