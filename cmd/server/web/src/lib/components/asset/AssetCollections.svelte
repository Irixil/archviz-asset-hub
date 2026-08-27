<script lang="ts">
  import { collectionApi, type Collection, type Asset } from '$lib/api'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { LibraryBig, X } from '@lucide/svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import { goto } from '$app/navigation'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { RemoveAssetFromCollection } from '$lib/commands/RemoveAssetFromCollection'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let memberships = $state<Collection[]>([])
  let loaded = $state(false)

  async function load() {
    try {
      memberships = await collectionApi.forAsset(asset.id)
      loaded = true
    } catch {
      memberships = []
    }
  }

  $effect(() => {
    asset.id
    loaded = false
    load()
  })

  async function handleRemove(col: Collection) {
    // Optimistic remove
    memberships = memberships.filter((c) => c.id !== col.id)
    try {
      await undoStore.execute(
        new RemoveAssetFromCollection(asset.id, col.id, col.name)
      )
    } catch {
      memberships = [...memberships, col]
      toastStore.show(m.save_failed(), 'error')
    }
  }

  function handleNavigate(col: Collection) {
    navigationStore.selectCollection(col.id)
    goto('/library')
  }
</script>

{#if loaded && memberships.length > 0}
  <div>
    <SubSectionTitle>{m.in_collections()}</SubSectionTitle>
    <div class="mt-2 flex flex-wrap gap-2">
      {#each memberships as col (col.id)}
        <div
          class="flex items-center gap-1 rounded-full border border-indigo-200 bg-indigo-50 px-2 py-0.5 text-sm dark:border-indigo-800 dark:bg-indigo-950"
        >
          <button
            type="button"
            onclick={() => handleNavigate(col)}
            class="flex items-center gap-1 text-indigo-700 hover:underline dark:text-indigo-300"
          >
            <LibraryBig class="h-3 w-3 shrink-0" />
            {col.name}
          </button>
          <button
            type="button"
            onclick={() => handleRemove(col)}
            class="ml-0.5 rounded-full p-0.5 text-indigo-400 hover:bg-indigo-100 hover:text-indigo-600 dark:hover:bg-indigo-900"
            aria-label="Remove from collection"
          >
            <X class="h-3 w-3" />
          </button>
        </div>
      {/each}
    </div>
  </div>
{/if}
