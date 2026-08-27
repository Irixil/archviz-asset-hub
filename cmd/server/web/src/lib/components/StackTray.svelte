<script lang="ts">
  import { stackStore } from '$lib/stores/stack.svelte'
  import { Layers, X } from '@lucide/svelte'
  import { slide } from 'svelte/transition'
  import StackPanel from './StackPanel.svelte'
  import { m } from '$lib/paraglide/messages'
  import { mimeCategory } from '$lib/api'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'

  const MAX_THUMBS = 5

  let confirmClear = $state(false)

  function handleClear() {
    if (confirmClear) {
      stackStore.clear()
      confirmClear = false
    } else {
      confirmClear = true
      setTimeout(() => {
        confirmClear = false
      }, 2500)
    }
  }
</script>

{#if stackStore.count > 0}
  <div
    transition:slide={{ duration: 200, axis: 'y' }}
    class="fixed right-0 bottom-[calc(var(--mobile-nav-height,64px)+0.5rem)] left-0 z-40 flex h-14 items-center gap-3 border-t border-amber-200 bg-amber-50 px-4 opacity-[92%] shadow-lg md:bottom-10 md:left-64 dark:border-amber-800 dark:bg-amber-950"
  >
    <!-- Thumbnail strip -->
    <div class="flex items-center -space-x-2">
      {#each stackStore.assets.slice(0, MAX_THUMBS) as asset (asset.id)}
        <div
          class="h-8 w-8 shrink-0 overflow-hidden rounded-md border-2 border-amber-100 dark:border-amber-900 {ASSET_BACKGROUND_COLORS[
            mimeCategory(asset.mimeType)
          ] ?? ''}"
          title={asset.name}
        >
          {#if asset.thumbnailUrl}
            <img
              src={asset.thumbnailUrl}
              alt={asset.name}
              class="h-full w-full object-cover"
            />
          {:else}
            <div
              class="flex h-full w-full items-center justify-center bg-amber-200 dark:bg-amber-800"
            >
              <Layers class="h-4 w-4 text-amber-600 dark:text-amber-400" />
            </div>
          {/if}
        </div>
      {/each}
      {#if stackStore.count > MAX_THUMBS}
        <div
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border-2 border-amber-100 bg-amber-200 text-xs font-semibold text-amber-700 dark:border-amber-900 dark:bg-amber-800 dark:text-amber-300"
        >
          +{stackStore.count - MAX_THUMBS}
        </div>
      {/if}
    </div>

    <!-- Stack label / count -->
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm font-semibold text-amber-800 hover:bg-amber-100 dark:text-amber-200 dark:hover:bg-amber-900"
      onclick={() => stackStore.togglePanel()}
    >
      <Layers class="h-4 w-4" />
      {stackStore.label ?? m.stack()} ({stackStore.count})
    </button>

    <!-- Spacer -->
    <div class="flex-1"></div>

    <!-- Clear button -->
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-sm font-medium transition-colors
        {confirmClear
        ? 'bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900 dark:text-red-300'
        : 'text-amber-700 hover:bg-amber-100 dark:text-amber-300 dark:hover:bg-amber-900'}"
      onclick={handleClear}
    >
      <X class="h-4 w-4" />
      {confirmClear
        ? m.clear_items({ count: stackStore.count })
        : m.clear_all()}
    </button>
  </div>
{/if}

{#if stackStore.panelOpen}
  <StackPanel />
{/if}
