<script lang="ts">
  import Button from '$lib/components/ui/Button.svelte'
  import type { Workflow } from '$lib/api/workflows'

  interface Props {
    workflow: Workflow
    active?: boolean
    busy?: boolean
    onSelect?: () => void
    onRun?: () => void
    onToggle?: () => void
  }

  let {
    workflow,
    active = false,
    busy = false,
    onSelect = () => {},
    onRun = () => {},
    onToggle = () => {},
  }: Props = $props()

  function relativeTime(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    const days = Math.floor(hrs / 24)
    if (days < 7) return `${days}d ago`
    return new Date(iso).toLocaleDateString()
  }
</script>

<div
  role="button"
  tabindex="0"
  data-trigger-type={workflow.trigger_type}
  class="w-full rounded-2xl border p-4 text-left transition-colors {active
    ? 'border-[var(--accent)] bg-[var(--accent-soft)]'
    : 'border-[var(--border-subtle)] bg-[var(--bg-surface)] hover:border-[var(--accent)]/40 hover:bg-[var(--bg-hover)]'}"
  onclick={onSelect}
  onkeydown={(e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSelect()
    }
  }}
>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <h3 class="truncate text-sm font-semibold text-[var(--text-primary)]">
          {workflow.name}
        </h3>
        <span
          class="rounded-full px-2 py-0.5 text-[10px] font-semibold tracking-[0.14em] uppercase {workflow.enabled
            ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
            : 'bg-zinc-500/10 text-zinc-500 dark:text-zinc-400'}"
        >
          {workflow.enabled ? 'Enabled' : 'Paused'}
        </span>
      </div>
      {#if workflow.description}
        <p class="mt-1.5 line-clamp-1 text-xs text-[var(--text-secondary)]">
          {workflow.description}
        </p>
      {/if}
      {#if workflow.last_run_at}
        <p class="mt-1.5 text-[11px] text-[var(--text-muted)]">
          Last run {relativeTime(workflow.last_run_at)}
        </p>
      {/if}
    </div>

    <div class="flex shrink-0 items-center gap-1.5">
      <Button
        size="sm"
        variant="ghost"
        disabled={busy}
        onclick={(e) => {
          e.stopPropagation()
          onToggle()
        }}
      >
        {workflow.enabled ? 'Pause' : 'Enable'}
      </Button>
      <Button
        size="sm"
        variant="outline"
        disabled={busy || !workflow.enabled}
        onclick={(e) => {
          e.stopPropagation()
          onRun()
        }}
      >
        Run
      </Button>
    </div>
  </div>
</div>
