<script lang="ts">
  import { assetApi, type Asset } from '$lib/api'
  import { Download } from '@lucide/svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import { m } from '$lib/paraglide/messages'

  type Props = {
    asset: Asset
  }
  let { asset }: Props = $props()
  let exportFormat = $state('original')
</script>

<div>
  <SubSectionTitle>{m.export()}</SubSectionTitle>
  <div class="flex gap-2">
    <select
      bind:value={exportFormat}
      class="text-md flex-1 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-gray-700 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
    >
      <option value="original">Original</option>
      <option value="jpeg">JPEG</option>
      <option value="png">PNG</option>
      <option value="webp">WebP</option>
      <option value="pdf">PDF</option>
    </select>
    <a
      href={assetApi.fileUrl(asset.id)}
      download={asset.original_filename}
      class="flex items-center justify-center rounded-xl bg-indigo-600 px-3.5 py-2.5 text-white transition-colors hover:bg-indigo-700"
      aria-label="Download"
    >
      <Download class="h-4 w-4" />
    </a>
  </div>
</div>
