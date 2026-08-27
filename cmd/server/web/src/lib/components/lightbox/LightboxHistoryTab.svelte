<script lang="ts">
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { Upload } from '@lucide/svelte'
  import VersionHistory from '$lib/components/VersionHistory.svelte'
  import UploadVersionModal from '$lib/components/UploadVersionModal.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    onVersionChanged: (updated: Asset) => void
  }

  let { asset, onVersionChanged }: Props = $props()

  let showUploadVersionModal = $state(false)
</script>

<div class="flex flex-col">
  {#if authStore.role !== 'viewer'}
    <div class="border-b border-gray-100 px-5 py-3 dark:border-gray-800">
      <button
        type="button"
        class="text-md flex w-full items-center justify-center gap-2 rounded-xl border border-indigo-200 px-4 py-2 font-medium text-indigo-600 transition-colors hover:bg-indigo-50 dark:border-indigo-700 dark:text-indigo-400 dark:hover:bg-indigo-900/20"
        onclick={() => {
          showUploadVersionModal = true
        }}
      >
        <Upload class="h-4 w-4" />
        {m.upload_new_version()}
      </button>
    </div>
  {/if}
  <VersionHistory
    {asset}
    onVersionChanged={(updated) => {
      onVersionChanged(updated)
    }}
  />
</div>

{#if showUploadVersionModal}
  <UploadVersionModal
    {asset}
    onclose={() => {
      showUploadVersionModal = false
    }}
    onuploaded={(updated) => {
      onVersionChanged(updated)
      showUploadVersionModal = false
    }}
  />
{/if}
