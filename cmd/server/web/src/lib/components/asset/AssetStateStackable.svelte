<script>
  import { assetApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { assetToStack, stackStore } from '$lib/stores/stack.svelte'
  import { Layers } from '@lucide/svelte'

  let { asset } = $props()
  let thumbUrl = $derived(assetApi.thumbUrl(asset.id))
  let stacked = $derived(stackStore.inStack(asset.id))
</script>

<div
  role="button"
  tabindex="0"
  class="absolute top-1.5 right-1.5 z-20 flex cursor-pointer items-center justify-center rounded-md p-1 transition-colors transition-opacity {stacked
    ? 'bg-amber-400 text-white opacity-100 shadow'
    : 'bg-black/40 text-white opacity-0 group-hover:opacity-100'}"
  onclick={(e) => {
    e.stopPropagation()
    stackStore.toggle(assetToStack(asset, thumbUrl))
  }}
  onkeydown={(e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.stopPropagation()
      stackStore.toggle(assetToStack(asset, thumbUrl))
    }
  }}
  title={stacked ? m.remove_from_stack() : m.add_to_stack()}
>
  <Layers class="h-3.5 w-3.5" />
</div>
