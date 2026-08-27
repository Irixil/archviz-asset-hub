<script lang="ts">
  import type { Snippet } from 'svelte'
  import { ChevronDown } from '@lucide/svelte'

  interface Props {
    title: string
    collapsible?: boolean
    initiallyCollapsed?: boolean
    children: Snippet
  }

  let {
    title,
    collapsible = false,
    initiallyCollapsed = false,
    children,
  }: Props = $props()

  let open = $state(false)

  $effect(() => {
    open = !initiallyCollapsed
  })
</script>

<section
  class="rounded-2xl border border-[var(--border-subtle)] bg-[var(--bg-panel)]/80"
>
  <button
    type="button"
    class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
    onclick={() => {
      if (collapsible) open = !open
    }}
    aria-expanded={collapsible ? open : undefined}
  >
    <span
      class="text-xs font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase"
    >
      {title}
    </span>
    {#if collapsible}
      <ChevronDown
        class={`h-4 w-4 text-[var(--text-muted)] transition-transform ${open ? '' : '-rotate-90'}`}
      />
    {/if}
  </button>

  {#if open}
    <dl
      class="grid grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)] gap-x-4 gap-y-2 border-t border-[var(--border-subtle)] px-4 py-4"
    >
      {@render children()}
    </dl>
  {/if}
</section>
