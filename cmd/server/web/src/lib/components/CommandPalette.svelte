<script lang="ts">
  import { type MenuItem, type Project } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { ChevronUp, Search } from '@lucide/svelte'

  interface Props {
    projects: Project[]
    onselect: (item: MenuItem | null) => void
    onclose: () => void
  }

  let { projects, onselect, onclose }: Props = $props()

  let query = $state('')
  let activeIndex = $state(0)
  let inputEl = $state<HTMLInputElement | undefined>(undefined)

  let items = $derived<MenuItem[]>(
    [
      { id: null, label: m.all_assets(), url: '/library' },
      { id: null, label: m.all_shares(), url: '/library/shares' },
      ...projects
        .filter((p) => p.name.toLowerCase().includes(query.toLowerCase()))
        .map((p) => ({
          id: p.id,
          label: p.name,
          color: p.color ?? '#9ca3af',
          count: p.asset_count,
        })),
    ].filter(
      (item) =>
        item.id === null ||
        query === '' ||
        item.label.toLowerCase().includes(query.toLowerCase())
    )
  )

  $effect(() => {
    activeIndex = 0
  })

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      activeIndex = Math.min(activeIndex + 1, items.length - 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      activeIndex = Math.max(activeIndex - 1, 0)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (items[activeIndex]) {
        onselect(items[activeIndex])
        onclose()
      }
    } else if (e.key === 'Escape') {
      onclose()
    }
  }

  $effect(() => {
    inputEl?.focus()
  })
</script>

<!-- Backdrop -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-start justify-center bg-black/40 pt-[20vh]"
  onclick={onclose}
  onkeydown={(e) => e.key === 'Escape' && onclose()}
>
  <!-- Panel -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="mx-4 w-full max-w-lg overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl"
    onclick={(e) => e.stopPropagation()}
    onkeydown={handleKeydown}
  >
    <!-- Input -->
    <div class="flex items-center gap-3 border-b border-gray-100 px-4 py-3">
      <Search class="h-5 w-5 text-gray-400" />
      <input
        bind:this={inputEl}
        bind:value={query}
        placeholder={m.jump_project()}
        class="text-md flex-1 placeholder-gray-400 outline-none"
      />
      <kbd
        class="rounded border border-gray-200 px-1.5 py-0.5 text-sm text-gray-400"
        >Esc</kbd
      >
    </div>

    <!-- Results -->
    <ul class="max-h-72 overflow-y-auto py-1">
      {#each items as item, i}
        <li>
          <button
            class="text-md flex w-full items-center gap-3 px-4 py-2.5 transition-colors {i ===
            activeIndex
              ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
              : 'text-gray-700 hover:bg-gray-50'}"
            onclick={() => {
              onselect(item)
              onclose()
            }}
            onmouseenter={() => {
              activeIndex = i
            }}
          >
            {#if item.color}
              <span
                class="h-2.5 w-2.5 shrink-0 rounded-full"
                style="background-color: {item.color}"
              ></span>
            {:else}
              <ChevronUp class="h-4 w-4 shrink-0 text-gray-400" />
            {/if}
            <span class="flex-1 text-left">{item.label}</span>
            {#if item.count !== undefined}
              <span class="text-sm text-gray-400">{item.count}</span>
            {/if}
          </button>
        </li>
      {/each}
      {#if items.length === 0}
        <li class="text-md px-4 py-3 text-gray-400">{m.no_results()}</li>
      {/if}
    </ul>
  </div>
</div>
