<script lang="ts">
  import { Trash2, X } from '@lucide/svelte'
  import Spinner from './Spinner.svelte'
  import type { Snippet } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  let {
    x = false,
    size = 4,
    loading = false,
    onclick,
    title = m.delete(),
    class: extraClass,
    children,
  }: {
    size?: number
    x?: boolean
    onclick: () => void
    loading?: boolean
    title?: string
    class?: string
    children?: Snippet
  } = $props()
</script>

<button
  type="button"
  class="flex h-8 w-8 items-center justify-center rounded text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400 {extraClass}"
  {onclick}
  {title}
>
  {#if loading}
    <Spinner size="sm" />
  {:else if children}
    {@render children()}
  {:else if x}
    <X class="h-{size} w-{size}" />
  {:else}
    <Trash2 class="h-{size} w-{size}" />
  {/if}
</button>
