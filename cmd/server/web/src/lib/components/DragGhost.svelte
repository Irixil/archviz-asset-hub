<script lang="ts">
  import { assetApi } from '$lib/api'
  import { onMount } from 'svelte'

  interface Props {
    assetIds: string[]
  }
  let { assetIds }: Props = $props()

  const visible = $derived(assetIds.reverse().slice(0, 3))
  const count = $derived(assetIds.length)

  const TRANSFORMS_START = [
    '-translate-y-6 -translate-x-10 -rotate-[30deg] scale-[1.4]',
    'rotate-[30deg] -translate-y-6 translate-x-10 scale-[1.4]',
    'rotate-0',
  ]
  const TRANSFORMS_END = [
    '-translate-x-1 -rotate-[10deg] scale-[1.1]',
    'rotate-[10deg] translate-x-1 scale-[1.1]',
    'rotate-0',
  ]

  let settled = $state(false)

  onMount(() => {
    requestAnimationFrame(() => {
      settled = true
    })
  })
</script>

<div class="relative h-28 w-28 p-4">
  {#each visible as id, i}asset-card
    <div
      class="drag-ghost-card absolute inset-4 overflow-hidden rounded-xl border-2 border-white/30 shadow-lg
             transition-transform duration-400 ease-out
             {settled ? TRANSFORMS_END[i] : TRANSFORMS_START[i]}"
      style="z-index: {i}"
    >
      <img
        src={assetApi.thumbUrl(id)}
        alt=""
        class="h-full w-full object-cover"
      />
    </div>
  {/each}

  {#if count > 1}
    <div
      class="absolute top-1 right-1 z-10 flex h-6 min-w-6 items-center justify-center rounded-full bg-indigo-600 px-1.5 text-xs font-bold text-white shadow"
    >
      {count}
    </div>
  {/if}
</div>
