<script lang="ts">
  import { onMount } from 'svelte'
  import { Plus, Upload } from '@lucide/svelte'
  import { exportsApi, type ExportConfig } from '$lib/api/exports'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import ExportConfigCard from '$lib/components/exports/ExportConfigCard.svelte'
  import ExportConfigModal from '$lib/components/exports/ExportConfigModal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'

  let configs = $state<ExportConfig[]>([])
  let loading = $state(true)
  let showModal = $state(false)
  let editingConfig = $state<ExportConfig | null>(null)
  let confirmDeleteConfig = $state<ExportConfig | null>(null)

  onMount(async () => {
    projectsStore.load()
    try {
      configs = await exportsApi.list()
    } finally {
      loading = false
    }
  })

  function openCreate() {
    editingConfig = null
    showModal = true
  }

  function openEdit(config: ExportConfig) {
    editingConfig = config
    showModal = true
  }

  function handleSaved(config: ExportConfig) {
    const idx = configs.findIndex((c) => c.id === config.id)
    if (idx >= 0) {
      configs[idx] = config
    } else {
      configs = [...configs, config]
    }
    showModal = false
  }

  function handleDelete(config: ExportConfig) {
    confirmDeleteConfig = config
  }

  async function confirmDelete() {
    if (!confirmDeleteConfig) return
    await exportsApi.delete(confirmDeleteConfig.id)
    configs = configs.filter((c) => c.id !== confirmDeleteConfig!.id)
    confirmDeleteConfig = null
  }

  async function handleToggle(config: ExportConfig) {
    const updated = await exportsApi.update(config.id, {
      enabled: !config.enabled,
    })
    const idx = configs.findIndex((c) => c.id === config.id)
    if (idx >= 0) configs[idx] = updated
  }
</script>

<svelte:head>
  <title>素材导出｜建筑素材中台</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader
    title="素材导出"
    description="按计划或按需将项目素材同步到指定位置。"
  >
    <Button variant="primary" onclick={openCreate}>
      {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
      新建导出任务
    </Button>
  </PageHeader>

  <div class="flex-1 overflow-y-auto px-6 py-6">
    {#if loading}
      <GridSkeleton lines={3} />
    {:else if configs.length === 0}
      <EmptyState
        title="暂无导出任务"
        description="新建导出任务，将项目素材同步到 SFTP 服务器或 Google Drive 文件夹。"
      >
        {#snippet action()}
          <Button variant="primary" onclick={openCreate}>
            {#snippet icon()}<Upload class="h-4 w-4" />{/snippet}
            新建导出任务
          </Button>
        {/snippet}
      </EmptyState>
    {:else}
      <div class="max-w-2xl space-y-3">
        {#each configs as config (config.id)}
          <ExportConfigCard
            {config}
            onEdit={openEdit}
            onDelete={handleDelete}
            onToggle={handleToggle}
          />
        {/each}
      </div>
    {/if}
  </div>
</div>

<ExportConfigModal
  bind:open={showModal}
  config={editingConfig}
  onSave={handleSaved}
  onClose={() => (showModal = false)}
/>

{#if confirmDeleteConfig}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
  >
    <div
      class="mx-4 w-full max-w-sm rounded-xl border border-gray-100 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900"
    >
      <p class="mb-2 font-semibold text-gray-900 dark:text-gray-100">
        删除导出任务？
      </p>
      <p class="mb-5 text-sm text-gray-500 dark:text-gray-400">
        已写入目标位置的文件不会受到影响。
      </p>
      <div class="flex justify-end gap-2">
        <Button
          variant="secondary"
          size="sm"
          onclick={() => (confirmDeleteConfig = null)}
        >
          取消
        </Button>
        <Button variant="danger" size="sm" onclick={confirmDelete}>删除</Button>
      </div>
    </div>
  </div>
{/if}
