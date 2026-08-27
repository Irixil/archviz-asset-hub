<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { uploadsStore } from '$lib/stores/uploads.svelte'
  import { assetApi } from '$lib/api'
  import { toastStore } from '$lib/stores/toast.svelte'
  import DuplicateWarningBanner from './duplicates/DuplicateWarningBanner.svelte'
  import { SvelteSet } from 'svelte/reactivity'

  let dismissedDuplicates = $state(new SvelteSet<string>())

  function keepBoth(uploadId: string) {
    dismissedDuplicates = new SvelteSet([...dismissedDuplicates, uploadId])
  }

  async function deleteThisCopy(uploadId: string, assetId: string) {
    try {
      await assetApi.delete(assetId)
      dismissedDuplicates = new SvelteSet([...dismissedDuplicates, uploadId])
      uploadsStore.remove(uploadId)
      toastStore.show(m.duplicate_banner_deleted())
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.duplicate_banner_delete_failed(),
        'error'
      )
    }
  }
</script>

{#if uploadsStore.items.length > 0}
  <div class="fixed right-1 bottom-4 z-50 transform">
    <div class="w-[300px] rounded-lg bg-white p-4 shadow-lg dark:bg-gray-800">
      <h3 class="text-md mb-2 font-semibold text-gray-400">{m.uploads()}</h3>
      <ul
        class="max-h-60 divide-y divide-gray-200 overflow-y-auto dark:divide-gray-700"
      >
        {#each uploadsStore.items as item (item.id)}
          <li class="py-2">
            <div class="flex items-center gap-3">
              <div class="min-w-0 flex-1">
                <p
                  class="truncate text-sm font-medium text-gray-800 dark:text-gray-200"
                >
                  {item.file.name}
                </p>
                {#if item.status === 'uploading'}
                  <div
                    class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700"
                  >
                    <div
                      class="h-full rounded-full bg-blue-500 transition-all"
                      style="width: {item.progress}%"
                    ></div>
                  </div>
                {/if}
              </div>
              {#if item.status === 'uploading'}
                <span class="text-sm text-gray-500 dark:text-gray-400"
                  >{item.progress}%</span
                >
              {:else if item.status === 'done'}
                <span class="text-sm text-green-500">{m.upload_done()}</span>
              {:else if item.status === 'error'}
                <span class="text-sm text-red-500">{m.error()}</span>
              {/if}
            </div>
            {#if item.asset?.duplicate_of && !dismissedDuplicates.has(item.id)}
              <div class="mt-2">
                <DuplicateWarningBanner
                  duplicate={item.asset.duplicate_of}
                  onKeepBoth={() => keepBoth(item.id)}
                  onDeleteThisCopy={() =>
                    deleteThisCopy(item.id, item.asset!.id)}
                />
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  </div>
{/if}
