<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import { ArrowLeft, SquarePen } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import RunDetail from '$lib/components/workflows/RunDetail.svelte'
  import {
    workflowsApi,
    type Workflow,
    type WorkflowGraph,
    type WorkflowNodeSchema,
    type WorkflowRun,
  } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'

  const runId = page.params.runId || ''
  const workflowId = page.url.searchParams.get('workflow') ?? ''

  let loading = $state(true)
  let run = $state<WorkflowRun | null>(null)
  let workflow = $state<Workflow | null>(null)
  let nodeSchemas = $state<WorkflowNodeSchema[]>([])

  onMount(() => {
    load()
  })

  async function load() {
    loading = true
    try {
      const [runData, schemasData, workflowData] = await Promise.all([
        workflowsApi.getRun(workflowId, runId),
        workflowsApi.getNodeSchemas(),
        workflowsApi.get(workflowId).catch(() => null),
      ])
      run = runData
      nodeSchemas = schemasData
      workflow = workflowData
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.load_page_failed(),
        'error'
      )
    } finally {
      loading = false
    }
  }

  async function refresh() {
    if (!workflowId || !runId) return
    try {
      run = await workflowsApi.getRun(workflowId, runId)
    } catch {
      // Ignore — run may have just completed.
    }
  }

  function parseGraph(wf: Workflow): WorkflowGraph {
    try {
      const parsed = JSON.parse(wf.graph) as WorkflowGraph
      return {
        nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
        edges: Array.isArray(parsed.edges) ? parsed.edges : [],
      }
    } catch {
      return { nodes: [], edges: [] }
    }
  }
</script>

<svelte:head>
  <title>{m.workflow_run_inspect()} — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader title={m.workflow_run_inspect()}>
    {#snippet meta()}
      {#if run}
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <span class="font-mono text-[11px] text-[var(--text-muted)]"
            >{m.workflow_run_entity()} {run.id.slice(0, 8)}</span
          >
          <span
            class="inline-flex rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {run.status ===
            'completed'
              ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
              : run.status === 'failed'
                ? 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
                : run.status === 'running'
                  ? 'bg-sky-500/10 text-sky-700 dark:text-sky-300'
                  : 'bg-slate-500/10 text-slate-700 dark:text-slate-300'}"
          >
            {run.status}
          </span>
          {#if run.started_at}
            <span class="text-[11px] text-[var(--text-muted)]">
              {m.started()}
              <span class="text-[var(--text-secondary)]"
                >{new Date(run.started_at).toLocaleString()}</span
              >
            </span>
          {/if}
          {#if run.completed_at}
            <span class="text-[11px] text-[var(--text-muted)]">
              {m.completed()}
              <span class="text-[var(--text-secondary)]"
                >{new Date(run.completed_at).toLocaleString()}</span
              >
            </span>
          {/if}
        </div>
      {/if}
    {/snippet}
    <div class="flex items-center gap-2">
      <Button
        variant="ghost"
        onclick={() => void goto('/library/settings/workflows/runs')}
      >
        {#snippet icon()}<ArrowLeft class="h-4 w-4" />{/snippet}
        Back
      </Button>
      {#if workflow}
        <Button
          variant="ghost"
          onclick={() =>
            void goto(`/library/settings/workflows?workflow=${workflowId}`)}
        >
          {#snippet icon()}<SquarePen class="h-4 w-4" />{/snippet}
          {m.workflow_edit()}
        </Button>
      {/if}
    </div>
  </PageHeader>

  <div class="flex-1 overflow-y-auto p-6">
    {#if loading}
      <GridSkeleton lines={6} />
    {:else}
      <RunDetail
        {run}
        graph={workflow ? parseGraph(workflow) : null}
        schemas={nodeSchemas}
        onRefresh={refresh}
      />
    {/if}
  </div>
</div>
