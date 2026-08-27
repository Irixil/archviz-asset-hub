<script lang="ts">
  import { onMount } from 'svelte'
  import { AlertCircle, RefreshCw } from '@lucide/svelte'
  import { workflowsApi, type Workflow } from '$lib/api/workflows'
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'

  interface AppliedWorkflow {
    workflowId: string
    workflowName: string
    runIds: string[]
    partialError: string | undefined
  }

  interface Props {
    assetIds: string[]
    onApplied?: (results: AppliedWorkflow[]) => void
  }

  let { assetIds, onApplied }: Props = $props()

  let workflows = $state<Workflow[]>([])
  let checkedIds = $state(new Set<string>())
  let loading = $state(true)
  let loadError = $state(false)
  let applying = $state(false)
  let applyError = $state<'none' | 'partial' | 'all'>('none')

  const selectedCount = $derived(checkedIds.size)

  onMount(() => {
    void load()
  })

  async function load() {
    loading = true
    loadError = false
    try {
      workflows = await workflowsApi.listManual()
      checkedIds = new Set()
    } catch {
      loadError = true
    } finally {
      loading = false
    }
  }

  function toggle(id: string) {
    if (applying) return
    const next = new Set(checkedIds)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    checkedIds = next
    applyError = 'none'
  }

  async function apply() {
    if (selectedCount === 0 || applying) return
    applying = true
    applyError = 'none'
    const selected = workflows.filter((workflow) => checkedIds.has(workflow.id))
    const settled = await Promise.allSettled(
      selected.map((workflow) =>
        workflowsApi.triggerBulk(workflow.id, assetIds).then((res) => ({
          workflowId: workflow.id,
          workflowName: workflow.name,
          runIds: res.run_ids,
          partialError: res.error,
        }))
      )
    )
    const clean = settled
      .filter(
        (result): result is PromiseFulfilledResult<AppliedWorkflow> =>
          result.status === 'fulfilled' && !result.value.partialError
      )
      .map((result) => result.value)
    const hasRejected = settled.some((result) => result.status === 'rejected')
    const hasPartial = settled.some(
      (result) => result.status === 'fulfilled' && !!result.value.partialError
    )
    applying = false
    if (clean.length > 0) onApplied?.(clean)
    if (hasRejected || hasPartial) {
      applyError = clean.length > 0 || hasPartial ? 'partial' : 'all'
    }
  }
</script>

{#if loading}
  <div
    class="flex min-h-40 flex-col items-center justify-center gap-3 text-sm text-gray-500 dark:text-gray-400"
  >
    <Spinner size="md" />
    <p>{m.workflows_library_modal_loading()}</p>
  </div>
{:else if loadError}
  <div
    class="flex min-h-40 flex-col items-center justify-center gap-3 text-sm text-gray-600 dark:text-gray-300"
  >
    <AlertCircle class="h-5 w-5 text-red-500" />
    <p>{m.workflows_library_modal_error()}</p>
    <Button variant="secondary" size="sm" onclick={() => void load()}>
      {#snippet icon()}<RefreshCw class="h-4 w-4" />{/snippet}
      {m.retry()}
    </Button>
  </div>
{:else if workflows.length === 0}
  <div
    class="flex min-h-40 flex-col items-center justify-center gap-2 text-sm text-gray-600 dark:text-gray-300"
  >
    <p>{m.workflows_library_modal_empty()}</p>
    <a
      class="font-medium text-indigo-600 hover:underline dark:text-indigo-400"
      href="/library/settings/workflows"
    >
      {m.workflows_library_modal_empty_link()}
    </a>
  </div>
{:else}
  <div
    class="max-h-80 overflow-y-auto rounded-lg border border-gray-200 dark:border-gray-700"
  >
    {#each workflows as workflow (workflow.id)}
      <label
        class="flex cursor-pointer items-start gap-3 border-b border-gray-100 px-3 py-3 last:border-b-0 hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/60"
      >
        <input
          type="checkbox"
          class="mt-1 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
          checked={checkedIds.has(workflow.id)}
          disabled={applying}
          onchange={() => toggle(workflow.id)}
        />
        <span class="min-w-0">
          <span
            class="block truncate text-sm font-medium text-gray-900 dark:text-gray-100"
            >{workflow.name}</span
          >
          <span
            class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400"
            >{workflow.description}</span
          >
        </span>
      </label>
    {/each}
  </div>

  {#if applyError !== 'none'}
    <p
      class="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/40 dark:text-red-300"
    >
      {applyError === 'partial'
        ? m.workflows_library_apply_partial_error()
        : m.workflows_library_apply_error()}
    </p>
  {/if}

  <div class="mt-4 flex justify-end">
    <Button
      variant="primary"
      size="sm"
      onclick={() => void apply()}
      disabled={selectedCount === 0 || applying}
      loading={applying}
    >
      {m.workflows_library_modal_apply({
        workflows: selectedCount,
        assets: assetIds.length,
      })}
    </Button>
  </div>
{/if}
