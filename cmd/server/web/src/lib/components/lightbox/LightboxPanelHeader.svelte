<script lang="ts">
  import { type Asset } from '$lib/api'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { RenameAsset } from '$lib/commands/RenameAsset'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import { X } from '@lucide/svelte'

  interface Props {
    asset: Asset
    category: string
    onClose: () => void
    onAssetRenamed: (updated: Asset) => void
  }

  let { asset, category, onClose, onAssetRenamed }: Props = $props()

  let renamingAsset = $state(false)
  let renameBusy = $state(false)

  function stemOf(filename: string) {
    const dot = filename.lastIndexOf('.')
    return dot > 0 ? filename.slice(0, dot) : filename
  }

  function extOf(filename: string) {
    const dot = filename.lastIndexOf('.')
    return dot > 0 ? filename.slice(dot) : ''
  }

  async function submitRename(stem: string) {
    if (stem === stemOf(asset.original_filename)) {
      renamingAsset = false
      return
    }
    renameBusy = true
    try {
      const before = asset.original_filename
      await undoStore.execute(new RenameAsset(asset.id, before, stem))
      const updated = {
        ...asset,
        original_filename:
          assetsStore.assets.find((a) => a.id === asset.id)
            ?.original_filename ?? asset.original_filename,
      }
      onAssetRenamed(updated)
    } finally {
      renameBusy = false
      renamingAsset = false
    }
  }
</script>

<div
  class="damask-texture damask-texture-strong relative flex h-24 flex-shrink-0 items-center justify-center px-4 {ASSET_BACKGROUND_COLORS[
    category
  ]}"
>
  {#if renamingAsset}
    <div class="flex w-full items-center gap-1 pr-10">
      <InlineEditForm
        value={stemOf(asset.original_filename)}
        busy={renameBusy}
        onsubmit={submitRename}
        oncancel={() => (renamingAsset = false)}
        size="md"
        autofocus
      />
      <span class="flex-shrink-0 text-sm text-black/50 dark:text-white/40"
        >{extOf(asset.original_filename)}</span
      >
    </div>
  {:else}
    <button
      type="button"
      class="damask-asset-name w-full cursor-pointer pr-10 text-left text-base leading-snug font-semibold break-words text-[var(--text-primary)] hover:underline"
      title="Click to rename"
      onclick={() => (renamingAsset = true)}
    >
      {asset.original_filename}
    </button>
  {/if}
  <div class="absolute top-3 right-3 flex items-center gap-1.5">
    <button
      type="button"
      class="flex h-8 w-8 items-center justify-center rounded-lg text-black/50 transition-colors hover:bg-black/10 hover:text-black/80 dark:text-white/50 dark:hover:bg-black/20 dark:hover:text-white/90"
      onclick={onClose}
      aria-label="Close panel"
    >
      <X size={16} />
    </button>
  </div>
</div>
