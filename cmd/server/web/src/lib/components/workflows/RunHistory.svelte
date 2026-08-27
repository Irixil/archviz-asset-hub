<script lang="ts">
  import type { WorkflowRun } from '$lib/api/workflows'

  interface Props {
    runs: WorkflowRun[]
    selectedRunId?: string | null
    onSelect?: (run: WorkflowRun) => void
  }

  let { runs, selectedRunId = null, onSelect = () => {} }: Props = $props()

  function statusClass(status: string) {
    switch (status) {
      case 'completed':
        return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
      case 'failed':
        return 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
      default:
        return 'bg-amber-500/10 text-amber-700 dark:text-amber-300'
    }
  }

  function duration(run: WorkflowRun) {
    if (!run.started_at || !run.completed_at) return 'In progress'
    const start = new Date(run.started_at).getTime()
    const end = new Date(run.completed_at).getTime()
    const ms = Math.max(0, end - start)
    if (ms < 1000) return `${ms} ms`
    return `${(ms / 1000).toFixed(1)} s`
  }
</script>

<div
  class="rounded-[24px] border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
>
  {#if runs.length === 0}
    <div class="px-4 py-6 text-xs text-[var(--text-muted)]">No runs yet.</div>
  {:else}
    <div class="max-h-[480px] overflow-y-auto">
      {#each runs as run (run.id)}
        <button
          type="button"
          class="flex w-full items-center gap-3 border-b border-[var(--border-subtle)] px-4 py-3 text-left transition-colors last:border-b-0 {selectedRunId ===
          run.id
            ? 'bg-[var(--accent-soft)]'
            : 'hover:bg-[var(--bg-hover)]'}"
          onclick={() => onSelect(run)}
        >
          <span
            class="h-1.5 w-1.5 shrink-0 rounded-full {run.status === 'completed'
              ? 'bg-emerald-400'
              : run.status === 'failed'
                ? 'bg-rose-400'
                : 'bg-amber-400'}"
          ></span>
          <div class="min-w-0 flex-1">
            <div class="flex items-baseline justify-between gap-2">
              <p class="font-mono text-[11px] text-[var(--text-muted)]">
                {run.id.slice(0, 8)}
              </p>
              <p
                class="shrink-0 font-mono text-[11px] text-[var(--text-muted)]"
              >
                {duration(run)}
              </p>
            </div>
            <p class="mt-0.5 text-[11px] text-[var(--text-secondary)]">
              {new Date(run.created_at).toLocaleString()}
            </p>
          </div>

          <span
            class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {statusClass(
              run.status
            )}"
          >
            {run.status}
          </span>
        </button>
      {/each}
    </div>
  {/if}
</div>
