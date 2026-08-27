<script lang="ts">
  import type { Snippet } from 'svelte'

  let { children = undefined }: { children?: Snippet } = $props()
</script>

<p
  class="mb-3 text-xs font-semibold tracking-widest text-gray-400 uppercase dark:text-gray-500"
>
  {@render children?.()}
</p>
