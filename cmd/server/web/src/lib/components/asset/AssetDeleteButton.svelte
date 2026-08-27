<script lang="ts">
  import { Loader, Trash } from '@lucide/svelte'
  import { assetApi, type Asset } from '$lib/api'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import { m } from '$lib/paraglide/messages'

  type Props = {
    asset: Asset
    ondeleted: (id: string) => void
  }
  let { asset, ondeleted }: Props = $props()

  let deleting = $state(false)

  async function handleDelete() {
    if (!asset) return
    deleting = true
    try {
      await assetApi.delete(asset.id)
      ondeleted(asset.id)
    } finally {
      deleting = false
    }
  }
</script>

<div>
  <SubSectionTitle>{m.delete()}</SubSectionTitle>
  <button
    class="text-md flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-red-500 px-4 py-3 text-black transition-colors hover:bg-red-600 dark:border-red-700"
    onclick={handleDelete}
    disabled={deleting}
    aria-label="Delete asset"
    title={m.delete()}
  >
    {#if deleting}
      <Loader class="h-4 w-4 animate-spin" />
    {:else}
      <Trash class="h-4 w-4" />
    {/if}
    {m.delete_permanently()}
  </button>
</div>
