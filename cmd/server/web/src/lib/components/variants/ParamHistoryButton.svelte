<script lang="ts">
  import { tick } from 'svelte'
  import { Clock } from '@lucide/svelte'
  import { variantApi, type ParamHistoryEntry } from '$lib/api'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    variantType: string
    /** Called when the user picks an entry. The form applies the params. */
    onSelect: (params: Record<string, unknown>) => void
    /** Label formatter — provided by the parent so it knows the field names. */
    formatEntry: (params: Record<string, unknown>) => string
    disabled?: boolean
    /** Tailwind width class for the dropdown panel. custom_ffmpeg passes a wider one. */
    dropdownWidthClass?: string
    /** Renders entry text in monospace — used by custom_ffmpeg for command-like rows. */
    monospaceEntries?: boolean
  }

  let {
    variantType,
    onSelect,
    formatEntry,
    disabled = false,
    dropdownWidthClass = 'w-64',
    monospaceEntries = false,
  }: Props = $props()

  let open = $state(false)
  let loading = $state(false)
  let entries = $state<readonly ParamHistoryEntry[]>([])
  let fetched = $state(false)
  let container = $state<HTMLDivElement | undefined>(undefined)
  let triggerEl = $state<HTMLButtonElement | undefined>(undefined)

  $effect(() => {
    void variantType
    open = false
    entries = []
    fetched = false
  })

  async function handleClick() {
    if (!fetched) {
      loading = true
      try {
        entries = await variantApi.paramHistory(variantType)
        fetched = true
      } catch {
        toastStore.show(m.variant_param_history_load_error(), 'error')
      } finally {
        loading = false
      }
    }
    if (entries.length === 0) return
    open = !open
    if (open) {
      await tick()
      container?.querySelector<HTMLButtonElement>('.history-item')?.focus()
    }
  }

  function pick(params: Record<string, unknown>) {
    onSelect(params)
    open = false
  }

  function handleWindowClick(e: MouseEvent) {
    if (open && container && !container.contains(e.target as Node)) {
      open = false
    }
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      open = false
      triggerEl?.focus()
    }
  }

  function focusAdjacentEntry(event: KeyboardEvent, direction: -1 | 1) {
    const current = event.currentTarget as HTMLButtonElement
    const menu = current.closest('[role="menu"]')
    const items = Array.from(
      menu?.querySelectorAll<HTMLButtonElement>('.history-item') ?? []
    )
    const index = items.indexOf(current)
    if (index === -1 || items.length === 0) return

    event.preventDefault()
    items[(index + direction + items.length) % items.length]?.focus()
  }

  function handleItemKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') focusAdjacentEntry(e, 1)
    if (e.key === 'ArrowUp') focusAdjacentEntry(e, -1)
  }

  const isEmpty = $derived(fetched && entries.length === 0)
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleWindowKeydown} />

<div class="relative" bind:this={container}>
  <button
    bind:this={triggerEl}
    type="button"
    onclick={handleClick}
    disabled={disabled || isEmpty}
    aria-label={m.variant_param_history_button_label()}
    aria-haspopup="menu"
    aria-expanded={open}
    title={isEmpty ? m.variant_param_history_empty() : undefined}
    class="flex h-8 w-8 items-center justify-center rounded-md text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
  >
    {#if loading}
      <Spinner size="sm" />
    {:else}
      <Clock size={16} />
    {/if}
  </button>

  {#if open}
    <div
      role="menu"
      aria-label={m.variant_param_history_button_label()}
      class="history-menu absolute right-[-20px] z-50 mt-1 {dropdownWidthClass} max-h-72 overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] py-1 shadow-lg"
    >
      {#each entries as entry, i (i)}
        <button
          type="button"
          role="menuitem"
          class="history-item block w-full truncate px-3 py-2 text-left text-sm text-[var(--text-primary)] hover:bg-[var(--bg-hover)]"
          class:font-mono={monospaceEntries}
          title={formatEntry(entry.params as Record<string, unknown>)}
          onclick={() => pick(entry.params as Record<string, unknown>)}
          onkeydown={handleItemKeydown}
        >
          {formatEntry(entry.params as Record<string, unknown>)}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .history-menu {
    transform-origin: top right;
    animation: history-menu-in 140ms cubic-bezier(0.25, 1, 0.5, 1) both;
  }

  @keyframes history-menu-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-4px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .history-menu {
      animation: none;
    }
  }
</style>
