<script lang="ts">
  import { getContext } from 'svelte'
  import {
    ACTION_SHEET_KEY,
    type ActionSheetContext,
  } from '$lib/components/ActionSheet.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'

  export interface ContextMenuItem {
    label: string
    icon?: unknown
    onclick: () => void
    danger?: boolean
  }

  interface Props {
    items: ContextMenuItem[]
    onclose?: () => void
    class?: string
  }

  let { items, onclose, class: extraClass = '' }: Props = $props()
  const actionSheet = getContext<ActionSheetContext | undefined>(
    ACTION_SHEET_KEY
  )
  let openedTouchSheet = $state(false)

  $effect(() => {
    if (!viewportStore.isTouch || openedTouchSheet || !actionSheet) return

    openedTouchSheet = true
    actionSheet.open({
      items: items.map((item, index) => ({
        key: `${index}-${item.label}`,
        label: item.label,
        variant: item.danger ? 'danger' : 'default',
        onSelect: item.onclick,
      })),
      onClose: () => {
        onclose?.()
      },
    })
  })
</script>

{#if !viewportStore.isTouch}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="min-w-[140px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg
      dark:border-gray-700 dark:bg-gray-800 {extraClass}"
    onmouseleave={onclose}
  >
    {#each items as item}
      <button
        type="button"
        class="text-md items-left flex w-full gap-2 px-3 py-1.5 text-left
          {item.danger
          ? 'text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/30'
          : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700'}"
        onclick={item.onclick}
      >
        {item.label}
      </button>
    {/each}
  </div>
{/if}
