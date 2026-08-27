<script lang="ts">
  import { assetApi, type Project } from '$lib/api'
  import { goto } from '$app/navigation'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { BulkTagAsset } from '$lib/commands/BulkTagAsset'
  import { BulkAssignAssetToProject } from '$lib/commands/BulkAssignAssetToProject'
  import {
    Layers,
    Settings2,
    SquareArrowRightExit,
    Tag,
    Workflow,
  } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'
  import BulkMetadataModal from '$lib/components/BulkMetadataModal.svelte'
  import ApplyWorkflowModal from '$lib/components/workflows/ApplyWorkflowModal.svelte'
  import { m } from '$lib/paraglide/messages'
  import { stackStore, assetToStack } from '$lib/stores/stack.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'

  interface Props {
    selectedIds: Set<string>
    projects: Project[]
    ondone: () => void
    onclear: () => void
  }

  let { selectedIds, projects, ondone, onclear }: Props = $props()

  function promoteToStack() {
    const resolved = [...selectedIds].map((id) =>
      assetsStore.assets.find((a) => a.id === id)
    )
    const missing = resolved.filter((a) => a == null).length
    if (missing > 0)
      console.warn(
        `promoteToStack: ${missing} selected asset(s) not in loaded list — skipped`
      )
    const stackAssets = resolved
      .filter((a) => a != null)
      .map((a) => assetToStack(a, assetApi.thumbUrl(a.id)))
    stackStore.fromSelection(stackAssets)
    onclear()
  }

  let tagInput = $state('')
  let busy = $state(false)
  let activePanel = $state<'tags' | 'projects' | null>(null)
  let showDeleteConfirm = $state(false)
  let showBulkMetadata = $state(false)
  let showApplyWorkflow = $state(false)

  async function bulkTag() {
    const name = tagInput.trim().toLowerCase()
    if (!name || selectedIds.size === 0) return
    busy = true
    try {
      await undoStore.execute(new BulkTagAsset([...selectedIds], name))
      tagInput = ''
      ondone()
    } finally {
      activePanel = null
      busy = false
    }
  }

  async function bulkProject(projectId: string | null) {
    if (selectedIds.size === 0) return
    busy = true
    try {
      const ids = [...selectedIds]
      const beforeProjectIds = new Map(
        ids.map((id) => [
          id,
          assetsStore.assets.find((a) => a.id === id)?.project_id ?? null,
        ])
      )
      const afterProject = projects.find((p) => p.id === projectId) ?? null
      await undoStore.execute(
        new BulkAssignAssetToProject(
          ids,
          beforeProjectIds,
          projectId ?? '',
          afterProject?.name ?? null
        )
      )
      ondone()
    } finally {
      activePanel = null
      busy = false
    }
  }

  const selectedAssetNames = $derived(
    [...selectedIds].map(
      (id) =>
        assetsStore.assets.find((a) => a.id === id)?.original_filename ?? id
    )
  )

  async function bulkDelete() {
    if (selectedIds.size === 0) return
    showDeleteConfirm = true
  }

  async function confirmBulkDelete() {
    busy = true
    try {
      await assetApi.bulkDelete([...selectedIds])
      ondone()
    } finally {
      busy = false
    }
  }

  function handleWorkflowsApplied(
    results: Array<{
      workflowId: string
      workflowName: string
      runIds: string[]
    }>
  ) {
    for (const { workflowName, runIds } of results) {
      toastStore.show(
        m.workflows_library_run_started({
          name: workflowName,
          count: runIds.length,
        }),
        'success',
        {
          label: m.workflows_library_view_runs(),
          onClick: () => void goto(`/library/settings/workflows/runs`),
        }
      )
    }
  }
</script>

{#if selectedIds.size > 0}
  <div class="fixed bottom-10 left-1/2 z-30 -translate-x-1/2">
    <div
      class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-2.5 shadow-xl dark:border-gray-700 dark:bg-gray-900"
    >
      <span class="text-md font-medium text-gray-700 dark:text-gray-300"
        >{selectedIds.size} selected</span
      >
      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <!-- Tag -->
      <div class="relative">
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => {
            activePanel = activePanel === 'tags' ? null : 'tags'
          }}
        >
          {#snippet icon()}<Tag class="h-4 w-4" />{/snippet}
          {m.tag()}
        </Button>
        {#if activePanel === 'tags'}
          <form
            class="absolute bottom-full mb-2 flex gap-1"
            onsubmit={(e) => {
              e.preventDefault()
              bulkTag()
            }}
          >
            <input
              bind:value={tagInput}
              placeholder={m.tag_name()}
              class="text-md rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 shadow-lg focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            />
            <Button type="submit" variant="primary" size="sm" disabled={busy}
              >{m.add()}</Button
            >
          </form>
        {/if}
      </div>

      <!-- Edit fields (editor+) -->
      {#if authStore.role !== 'viewer'}
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => {
            activePanel = null
            showBulkMetadata = true
          }}
        >
          {#snippet icon()}<Settings2 class="h-4 w-4" />{/snippet}
          {m.bulk_action_edit_fields()}
        </Button>

        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => {
            activePanel = null
            showApplyWorkflow = true
          }}
        >
          {#snippet icon()}<Workflow class="h-4 w-4" />{/snippet}
          {m.workflows_library_apply_workflow()}
        </Button>
      {/if}

      <!-- Assign project -->
      <div class="relative">
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => {
            activePanel = activePanel === 'projects' ? null : 'projects'
          }}
        >
          {#snippet icon()}<SquareArrowRightExit class="h-4 w-4" />{/snippet}
          {m.project()}
        </Button>
        {#if activePanel === 'projects'}
          <div
            class="absolute bottom-full mb-2 min-w-[160px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
          >
            <button
              class="text-md flex w-full items-center gap-2 px-3 py-1.5 text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800"
              onclick={() => bulkProject(null)}
            >
              <span class="h-2.5 w-2.5 rounded-full border border-gray-300"
              ></span>
              {m.none()}
            </button>
            {#each projects as p}
              <button
                class="text-md flex w-full items-center gap-2 px-3 py-1.5 text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                onclick={() => bulkProject(p.id)}
              >
                <span
                  class="h-2.5 w-2.5 shrink-0 rounded-full"
                  style="background-color: {p.color ?? '#9ca3af'}"
                ></span>
                <span class="truncate">{p.name}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <!-- Delete (owner only) -->
      {#if authStore.role === 'owner'}
        <ButtonDelete title={m.bulk_delete_variants()} onclick={bulkDelete} />
      {/if}

      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <Button
        variant="ghost"
        size="sm"
        disabled={busy || stackStore.active}
        onclick={promoteToStack}
      >
        {#snippet icon()}<Layers class="h-4 w-4" />{/snippet}
        {m.stack_create()} ({selectedIds.size}) →
      </Button>

      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <Button variant="ghost" size="sm" onclick={onclear}>{m.cancel()}</Button>
    </div>
  </div>
{/if}

{#if showBulkMetadata}
  <BulkMetadataModal
    assetIds={[...selectedIds]}
    onClose={() => (showBulkMetadata = false)}
    onCommit={() => {
      showBulkMetadata = false
      ondone()
    }}
  />
{/if}

{#if showApplyWorkflow}
  <ApplyWorkflowModal
    assetIds={[...selectedIds]}
    onClose={() => (showApplyWorkflow = false)}
    onApplied={handleWorkflowsApplied}
  />
{/if}

<ConfirmModal
  bind:open={showDeleteConfirm}
  title={m.delete_n_assets({ count: selectedIds.size })}
  items={selectedAssetNames}
  onConfirm={confirmBulkDelete}
  onCancel={() => {
    showDeleteConfirm = false
  }}
/>
