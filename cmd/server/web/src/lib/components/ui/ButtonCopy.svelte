<script lang="ts">
  import { CircleCheck, Copy } from '@lucide/svelte'
  import type { Snippet } from 'svelte'
  let {
    title = 'Copy',
    text = '',
    class: extraClass,
    onclick,
    children,
    copied = false,
  }: {
    copied?: boolean
    text?: string
    onclick: () => void
    title?: string
    class?: string
    children?: Snippet
  } = $props()
</script>

<button
  type="button"
  class={`flex items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200 ${extraClass}`}
  {onclick}
  aria-label={title}
  {title}
>
  {#if copied}
    <CircleCheck class="h-4 w-4 text-green-500" />
  {:else if children}
    {@render children()}
  {:else}
    <Copy class="h-4 w-4" />
  {/if}
  {text}
</button>
