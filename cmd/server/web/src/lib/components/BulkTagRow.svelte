<script lang="ts">
  import { tagApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { Minus, Plus, Trash2 } from '@lucide/svelte'

  interface Props {
    mode: 'add' | 'remove'
    tag: string
    onRemoveRow: () => void
  }

  let {
    mode = $bindable<'add' | 'remove'>('add'),
    tag = $bindable(''),
    onRemoveRow,
  }: Props = $props()

  let allTags = $state<string[]>([])
  let suggestions = $state<string[]>([])
  let showSuggestions = $state(false)

  async function loadTags() {
    if (allTags.length) return
    try {
      const res = await tagApi.list()
      allTags = res.map((t) => t.name)
    } catch {
      /* ignore */
    }
  }

  function updateSuggestions() {
    const q = tag.trim().toLowerCase()
    if (!q) {
      suggestions = []
      return
    }
    suggestions = allTags.filter((n) => n.toLowerCase().includes(q)).slice(0, 5)
  }

  function pickSuggestion(name: string) {
    tag = name
    suggestions = []
    showSuggestions = false
  }
</script>

<div class="flex items-center gap-2">
  <button
    type="button"
    class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border transition-colors
      {mode === 'add'
      ? 'border-emerald-300 bg-emerald-50 text-emerald-600 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
      : 'border-rose-300 bg-rose-50 text-rose-600 hover:bg-rose-100 dark:border-rose-700 dark:bg-rose-900/30 dark:text-rose-400'}"
    aria-label={mode === 'add'
      ? m.bulk_tag_mode_add()
      : m.bulk_tag_mode_remove()}
    onclick={() => (mode = mode === 'add' ? 'remove' : 'add')}
  >
    {#if mode === 'add'}
      <Plus class="h-3.5 w-3.5" />
    {:else}
      <Minus class="h-3.5 w-3.5" />
    {/if}
  </button>

  <div class="relative flex-1">
    <input
      bind:value={tag}
      type="text"
      placeholder="tag name…"
      class="w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 focus:border-blue-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
      onfocus={() => {
        loadTags()
        showSuggestions = true
      }}
      onblur={() => setTimeout(() => (showSuggestions = false), 150)}
      oninput={updateSuggestions}
    />
    {#if showSuggestions && suggestions.length > 0}
      <ul
        class="absolute top-full z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
      >
        {#each suggestions as s}
          <li>
            <button
              type="button"
              class="w-full px-3 py-1.5 text-left text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
              onclick={() => pickSuggestion(s)}
            >
              {s}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <button
    type="button"
    class="flex h-6 w-6 shrink-0 items-center justify-center rounded text-gray-300 transition-colors hover:bg-rose-50 hover:text-rose-500 dark:text-gray-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-400"
    aria-label="Remove row"
    onclick={onRemoveRow}
  >
    <Trash2 class="h-3.5 w-3.5" />
  </button>
</div>
