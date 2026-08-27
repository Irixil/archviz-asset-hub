<script lang="ts">
  import { Merge } from '@lucide/svelte'
  import ButtonEdit from './ui/ButtonEdit.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import ButtonDelete from './ui/ButtonDelete.svelte'
  import ButtonCancel from './ui/ButtonCancel.svelte'
  import type { SvelteSet } from 'svelte/reactivity'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    selected: SvelteSet<string>
    ondelete: () => void
    onmerge: () => void
    onclear: () => void
  }

  let { selected, ondelete, onmerge, onclear }: Props = $props()
</script>

{#if selected.size > 0}
  <div class="fixed bottom-10 left-1/2 z-30 -translate-x-1/2">
    <div
      class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-2.5 shadow-xl dark:border-gray-700 dark:bg-gray-900"
    >
      <span class="text-md font-medium text-gray-700 dark:text-gray-300"
        >{selected.size} {m.selected()}</span
      >
      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      {#if selected.size >= 2}
        <ButtonEdit onclick={onmerge}>
          <Merge class="mr-1 h-3.5 w-3.5" />
          {m.merge_selected()}
        </ButtonEdit>

        <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>
      {/if}

      <!-- Delete (owner only) -->
      {#if authStore.role === 'owner'}
        <ButtonDelete onclick={ondelete} />

        <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>
      {/if}

      <ButtonCancel x onclick={onclear} />
    </div>
  </div>
{/if}
