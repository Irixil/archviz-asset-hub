<script lang="ts">
  import type { Snippet } from 'svelte'
  let { children, class: extraClass }: { children: Snippet; class?: string } =
    $props()
</script>

<div class={`flex h-full flex-col overflow-hidden ${extraClass}`}>
  {@render children()}
</div>
