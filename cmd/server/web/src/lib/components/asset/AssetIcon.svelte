<script lang="ts">
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'
  import { File, Image, Music, Play } from '@lucide/svelte'

  type Props = {
    category: 'image' | 'video' | 'audio' | 'document'
    class?: string
  }

  let { category, class: extraClass }: Props = $props()
</script>

<div
  class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg {ASSET_BACKGROUND_COLORS[
    category
  ]} {extraClass}"
>
  {#if category === 'image'}
    <Image class="h-5 w-5 text-white" />
  {:else if category === 'video'}
    <Play class="h-5 w-5 text-white" />
  {:else if category === 'audio'}
    <Music class="h-5 w-5 text-white" />
  {:else}
    <File class="h-5 w-5 text-white" />
  {/if}
</div>
