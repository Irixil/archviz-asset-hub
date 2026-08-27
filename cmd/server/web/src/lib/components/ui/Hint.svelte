<script lang="ts">
  import type { Snippet } from 'svelte'
  let {
    class: extraClass = '',
    children,
  }: { class?: string; children: Snippet } = $props()
</script>

<p class={`md:text-md text-sm text-gray-500 dark:text-gray-400 ${extraClass}`}>
  {@render children()}
</p>
