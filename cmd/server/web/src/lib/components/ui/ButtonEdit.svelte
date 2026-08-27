<script lang="ts">
  import { Pencil } from '@lucide/svelte'
  import type { Snippet } from 'svelte'
  import Spinner from './Spinner.svelte'
  import { m } from '$lib/paraglide/messages'

  let {
    disabled = false,
    loading = false,
    title = m.edit(),
    class: extraClass,
    onclick,
    children,
  }: {
    onclick: () => void
    disabled?: boolean
    loading?: boolean
    title?: string
    class?: string
    children?: Snippet
  } = $props()
</script>

<button
  type="button"
  class={`flex items-center justify-center rounded text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200 ${extraClass}`}
  {disabled}
  {onclick}
  {title}
>
  {#if loading}
    <Spinner size="sm" />
  {:else if children}
    {@render children()}
  {:else}
    <Pencil class="h-4 w-4" />
  {/if}
</button>
