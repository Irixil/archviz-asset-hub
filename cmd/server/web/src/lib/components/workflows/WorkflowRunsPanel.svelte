<script lang="ts">
  import { X, Search, RefreshCw } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { goto } from '$app/navigation'
  import {
    workflowsApi,
    type Workflow,
    type WorkflowRun,
  } from '$lib/api/workflows'
  import Button from '$lib/components/ui/Button.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'

  interface Props {
    workflow: Workflow
    onclose: () => void
  }

  let { workflow, onclose }: Props = $props()

  let runs = $state<WorkflowRun[]>([])
  let loading = $state(false)
  let selectedRunId = $state<string | null>(null)

  async function loadRuns() {
    loading = true
    try {
      const result = await workflowsApi.listRuns(workflow.id)
      runs = result
    } finally {
      loading = false
    }
  }

  $effect(() => {
    void workflow.id
    runs = []
    selectedRunId = null
    void loadRuns()
  })
</script>

<aside
  in:fly={{ x: 250, duration: 150, delay: 150 }}
  out:fly={{ x: 250, duration: 150 }}
  class="flex w-2xl shrink-0 flex-col border-l border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
>
  <!-- Header -->
  <div
    class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800"
  >
    <div class="min-w-0">
      <h2
        class="text-md truncate font-semibold text-gray-900 dark:text-gray-50"
      >
        {workflow.name}
      </h2>
      <Hint class="text-sm">Run history</Hint>
    </div>
    <div class="flex items-center gap-1">
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        onclick={loadRuns}
        title="Refresh"
      >
        <RefreshCw class="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        onclick={onclose}
      >
        <X class="h-4 w-4" />
      </button>
    </div>
  </div>

  <!-- Run list -->
  <div class="flex-1 overflow-y-auto p-4">
    {#if loading}
      <GridSkeleton lines={4} />
    {:else if runs.length === 0}
      <p class="py-10 text-center text-sm text-gray-400">No runs yet.</p>
    {:else}
      <div class="space-y-1.5">
        {#each runs as run (run.id)}
          {@const isSelected = selectedRunId === run.id}
          <div
            class="rounded-lg border transition-colors
              {isSelected
              ? 'border-violet-200 bg-violet-50 dark:border-violet-800 dark:bg-violet-900/20'
              : 'border-gray-100 bg-gray-50 hover:border-gray-200 dark:border-gray-800 dark:bg-gray-800/50 dark:hover:border-gray-700'}"
          >
            <button
              type="button"
              class="flex w-full items-center gap-3 px-3 py-2.5 text-left"
              onclick={() => {
                selectedRunId = isSelected ? null : run.id
              }}
            >
              <span
                class="h-1.5 w-1.5 shrink-0 rounded-full {run.status ===
                'completed'
                  ? 'bg-emerald-400'
                  : run.status === 'failed'
                    ? 'bg-rose-400'
                    : 'bg-amber-400'}"
              ></span>
              <div class="min-w-0 flex-1">
                <div class="flex items-baseline justify-between gap-2">
                  <p class="font-mono text-[11px] text-gray-400">
                    {run.id.slice(0, 8)}
                  </p>
                  <span
                    class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase
                      {run.status === 'completed'
                      ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                      : run.status === 'failed'
                        ? 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
                        : 'bg-amber-500/10 text-amber-700 dark:text-amber-300'}"
                  >
                    {run.status}
                  </span>
                </div>
                <p class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {new Date(run.created_at).toLocaleString()}
                </p>
              </div>
            </button>

            {#if isSelected}
              <div
                class="border-t border-gray-100 px-3 py-2 dark:border-gray-700"
              >
                <Button
                  size="sm"
                  variant="ghost"
                  onclick={() =>
                    goto(
                      `/library/settings/workflows/runs/${run.id}?workflow=${workflow.id}`
                    )}
                >
                  {#snippet icon()}<Search class="h-3 w-3" />{/snippet}
                  Inspect
                </Button>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</aside>
