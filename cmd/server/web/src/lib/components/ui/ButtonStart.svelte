<script lang="ts">
  import { Play } from '@lucide/svelte'
  let {
    onclick,
    title,
    disabled,
  }: { onclick: () => void; title: string; disabled: boolean } = $props()
</script>

<button
  type="button"
  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-gray-800 dark:hover:text-gray-200"
  {disabled}
  {onclick}
  {title}
>
  <Play class="h-4 w-4" />
</button>
