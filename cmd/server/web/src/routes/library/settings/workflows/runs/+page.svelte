<script lang="ts">
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'
  import { Activity } from '@lucide/svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { workflowsApi, type WorkflowRun } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'

  let loading = $state(true)
  let runs = $state<WorkflowRun[]>([])
  let workflowNames = $state<Map<string, string | null>>(new Map())

  onMount(() => {
    void load()
  })

  async function load() {
    loading = true
    try {
      const { runs: allRuns } = await workflowsApi.listAllRuns()
      runs = [...allRuns]
      // Resolve workflow names lazily — one request per unique workflow_id.
      // Deleted workflows resolve to null and show a fallback label.
      const ids = [...new Set(allRuns.map((r) => r.workflow_id))]
      const entries = await Promise.all(
        ids.map((id) =>
          workflowsApi
            .get(id)
            .then((wf) => [id, wf.name] as const)
            .catch(() => [id, null] as const)
        )
      )
      workflowNames = new Map(entries)
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.workflows_library_modal_error(),
        'error'
      )
    } finally {
      loading = false
    }
  }

  function workflowLabel(run: WorkflowRun): string {
    const name = workflowNames.get(run.workflow_id)
    if (name === undefined) return run.workflow_id.slice(0, 8)
    if (name === null) return `[deleted] ${run.workflow_id.slice(0, 8)}`
    return name
  }

  function isDeleted(run: WorkflowRun): boolean {
    return (
      workflowNames.has(run.workflow_id) &&
      workflowNames.get(run.workflow_id) === null
    )
  }

  function statusDot(status: string) {
    switch (status) {
      case 'completed':
        return 'bg-emerald-400'
      case 'failed':
        return 'bg-rose-400'
      default:
        return 'bg-amber-400'
    }
  }

  function statusBadge(status: string) {
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
    if (!run.started_at || !run.completed_at) return m.in_progress()
    const ms = Math.max(
      0,
      new Date(run.completed_at).getTime() - new Date(run.started_at).getTime()
    )
    if (ms < 1000) return `${ms} ms`
    return `${(ms / 1000).toFixed(1)} s`
  }

  function handleRowClick(run: WorkflowRun) {
    if (isDeleted(run)) return
    void goto(
      `/library/settings/workflows/runs/${run.id}?workflow=${run.workflow_id}`
    )
  }
</script>

<svelte:head>
  <title>{m.workflows_settings_tab_runs()} — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader
    title={m.workflows_settings_tab_runs()}
    description={m.workflows_runs_page_description()}
  />

  <div class="flex-1 overflow-y-auto px-6 py-6">
    {#if loading}
      <GridSkeleton lines={6} />
    {:else if runs.length === 0}
      <EmptyState
        title={m.workflows_runs_empty_title()}
        description={m.workflows_runs_empty_description()}
      >
        {#snippet icon()}
          <Activity class="h-10 w-10" />
        {/snippet}
      </EmptyState>
    {:else}
      <div
        class="overflow-hidden rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
      >
        {#each runs as run (run.id)}
          {@const deleted = isDeleted(run)}
          <button
            type="button"
            class="flex w-full items-center gap-4 border-b border-[var(--border-subtle)] px-5 py-3.5 text-left transition-colors last:border-b-0 {deleted
              ? 'cursor-default opacity-60'
              : 'hover:bg-[var(--bg-hover)]'}"
            onclick={() => handleRowClick(run)}
            disabled={deleted}
          >
            <span class="h-2 w-2 shrink-0 rounded-full {statusDot(run.status)}"
            ></span>

            <div class="min-w-0 flex-1">
              <div class="flex items-baseline gap-3">
                <p
                  class="truncate text-sm font-medium text-[var(--text-primary)]"
                >
                  {workflowLabel(run)}
                </p>
                <span class="font-mono text-[11px] text-[var(--text-muted)]">
                  {run.id.slice(0, 8)}
                </span>
              </div>
              <p class="mt-0.5 text-xs text-[var(--text-muted)]">
                {new Date(run.created_at).toLocaleString()}
              </p>
            </div>

            <div class="flex shrink-0 items-center gap-3">
              <span class="font-mono text-xs text-[var(--text-muted)]">
                {duration(run)}
              </span>
              <span
                class="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {statusBadge(
                  run.status
                )}"
              >
                {run.status}
              </span>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>
