<script lang="ts">
  import { ToggleLeft, ToggleRight } from '@lucide/svelte'
  let {
    enabled,
    onclick,
    title,
  }: { enabled: boolean; onclick: () => void; title: string } = $props()
</script>

<button
  type="button"
  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
  {onclick}
  {title}
>
  {#if enabled}
    <ToggleRight class="h-4 w-4 text-indigo-500" />
  {:else}
    <ToggleLeft class="h-4 w-4" />
  {/if}
</button>
