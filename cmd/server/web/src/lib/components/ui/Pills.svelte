<script lang="ts">
  type Pill = { id: string; label: string }
  type Props = {
    pills: Pill[]
    active: string
    set: (p: Pill) => void
  }
  let { pills, active, set }: Props = $props()
</script>

<div
  class="flex flex-wrap gap-1.5 border-b border-[var(--border-subtle)] px-5 py-2.5"
>
  {#each pills as p}
    <button
      type="button"
      class="pill-btn rounded-full px-3 py-1 text-xs font-medium transition-all duration-150 {active ===
      p.id
        ? 'active'
        : ''}"
      onclick={() => set(p)}
    >
      {p.label}
    </button>
  {/each}
</div>

<style>
  .pill-btn {
    background: var(--bg-elevated);
    color: var(--text-secondary);
  }
  .pill-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .pill-btn.active {
    background: oklch(93% 0.04 270);
    color: oklch(40% 0.18 270);
  }
  :global(.dark) .pill-btn.active {
    background: oklch(30% 0.08 270 / 0.45);
    color: oklch(78% 0.12 270);
  }
</style>
