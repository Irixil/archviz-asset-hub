<script lang="ts">
  import { X } from '@lucide/svelte'

  let { close } = $props()
</script>

<button
  type="button"
  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
  onclick={close}
  aria-label="Close panel"
>
  <X class="h-4 w-4" />
</button>
