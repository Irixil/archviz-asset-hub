<script lang="ts">
  import type { GridMode } from '$lib/stores/bottomStatusBar.svelte'
  import { Grid3X3, LayoutGrid, Rows3 } from '@lucide/svelte'

  let { mode, onchange }: { mode: GridMode; onchange: (m: GridMode) => void } =
    $props()
</script>

<div
  class="ml-auto flex items-center gap-0.5 rounded-md border border-[var(--border-default)] bg-[var(--bg-app)] p-0.5"
>
  <button
    type="button"
    aria-label="宽松网格"
    aria-pressed={mode === 'spaced'}
    onclick={() => onchange('spaced')}
    class="flex h-8 w-8 items-center justify-center rounded-sm transition-colors {mode ===
    'spaced'
      ? 'bg-[var(--selection-bar-bg)] text-[var(--selection-bar-text)] shadow-sm'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <LayoutGrid class="h-4 w-4" />
  </button>
  <button
    type="button"
    aria-label="紧凑网格"
    aria-pressed={mode === 'compact'}
    onclick={() => onchange('compact')}
    class="flex h-8 w-8 items-center justify-center rounded-sm transition-colors {mode ===
    'compact'
      ? 'bg-[var(--selection-bar-bg)] text-[var(--selection-bar-text)] shadow-sm'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <Grid3X3 class="h-4 w-4" />
  </button>
  <button
    type="button"
    aria-label="表格视图"
    aria-pressed={mode === 'table'}
    onclick={() => onchange('table')}
    class="flex h-8 w-8 items-center justify-center rounded-sm transition-colors {mode ===
    'table'
      ? 'bg-[var(--selection-bar-bg)] text-[var(--selection-bar-text)] shadow-sm'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <Rows3 class="h-4 w-4" />
  </button>
</div>
