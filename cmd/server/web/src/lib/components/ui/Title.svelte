<script lang="ts">
  import type { Snippet } from 'svelte'

  let { children }: { children: Snippet } = $props()
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-50">
  {@render children()}
</h1>
