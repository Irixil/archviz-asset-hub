<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import FolderTree from './FolderTree.svelte'
  import { Box, EllipsisVertical, Plus } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'
  import Feedback from './ui/Feedback.svelte'
  import { assetApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    selectedAssetIds: Set<string>
    creating: boolean
    onCreatingChange: (v: boolean) => void
    onSelect: (id: string | null) => void
    onFolderSelect: (projectId: string, folderId: string | null) => void
    onAssetsFolderDropped: (
      assetIds: string[],
      folderId: string | null,
      projectId: string
    ) => void
    onAssetsProjectDropped: (assetIds: string[], projectId: string) => void
  }

  let {
    selectedAssetIds,
    creating,
    onCreatingChange,
    onSelect,
    onFolderSelect,
    onAssetsFolderDropped,
    onAssetsProjectDropped,
  }: Props = $props()

  const emailIngestToken = $derived(
    ingressStore.sources.find((s) => s.type === 'email_api' && s.enabled)
      ?.public_token ?? null
  )

  let dropTargetProjectId = $state<string | null>(null)
  let creatingFolderForProject = $state<string | null>(null)
  let newFolderName = $state('')
  let newName = $state('')
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let menuOpenId = $state<string | null>(null)
  let error = $state('')

  async function submitCreate() {
    const name = newName.trim()
    if (!name) return
    error = ''
    try {
      await projectsStore.create({ name, color: '#b7771d' })
      newName = ''
      onCreatingChange(false)
    } catch {
      error = m.project_create_failed()
    }
  }

  async function submitEdit(id: string, name: string) {
    if (!name) return
    try {
      await projectsStore.update(id, { name })
      editingId = null
    } catch {
      error = m.project_rename_failed()
    }
  }

  async function deleteProject(id: string) {
    menuOpenId = null
    try {
      await projectsStore.delete(id)
    } catch {
      error = m.delete_project_failed()
    }
  }

  async function submitCreateFolder(projectId: string) {
    const name = newFolderName.trim()
    if (!name) {
      creatingFolderForProject = null
      return
    }
    try {
      await foldersStore.create(projectId, { name })
      newFolderName = ''
      creatingFolderForProject = null
    } catch {
      error = m.folder_create_failed()
    }
  }

  function startEdit(id: string, name: string) {
    editingId = id
    editName = name
    menuOpenId = null
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onCreatingChange(false)
      editingId = null
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-col gap-0.5">
  {#each projectsStore.projects as project (project.id)}
    <div class="group relative m-[1px]">
      {#if editingId === project.id}
        <div class="px-2 py-1">
          <InlineEditForm
            bind:value={editName}
            onsubmit={(v) => submitEdit(project.id, v)}
            oncancel={() => {
              editingId = null
            }}
            size="sm"
          />
        </div>
      {:else}
        <button
          aria-current={navigationStore.activeProjectId === project.id
            ? 'page'
            : undefined}
          class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors
            {navigationStore.activeProjectId === project.id
            ? 'bg-[var(--selection-bar-bg)] font-semibold text-[var(--selection-bar-text)] focus-visible:ring-[var(--accent)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] focus-visible:ring-[var(--accent)]'}
            {dropTargetProjectId === project.id
            ? 'ring-2 ring-[var(--accent)] ring-offset-1 ring-offset-[var(--bg-sidebar)]'
            : ''}"
          onclick={() => onSelect(project.id)}
          ondragover={(e) => {
            e.preventDefault()
            dropTargetProjectId = project.id
          }}
          ondragleave={() => {
            dropTargetProjectId = null
          }}
          ondrop={(e) => {
            e.preventDefault()
            dropTargetProjectId = null
            const assetId = e.dataTransfer?.getData('text/plain')
            if (!assetId) return
            const assetIds =
              selectedAssetIds.has(assetId) && selectedAssetIds.size > 1
                ? [...selectedAssetIds]
                : [assetId]
            onAssetsProjectDropped(assetIds, project.id)
          }}
        >
          {#if project.cover_asset_id}
            <img
              src={assetApi.thumbUrl(project.cover_asset_id)}
              class="h-4 w-4 rounded object-cover"
              alt={`${project.name}项目封面`}
            />
          {:else}
            <Box class="h-4 w-4 shrink-0" />
          {/if}
          <span class="min-w-0 flex-1 truncate text-left">{project.name}</span>
          <span
            class="ml-auto shrink-0 p-1 text-xs text-[var(--text-muted)] tabular-nums"
            >{project.asset_count || ''}</span
          >
        </button>

        {#if authStore.role !== 'viewer'}
          <button
            class="absolute top-2 right-7 rounded p-0.5 text-[var(--text-muted)] opacity-100 transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)] md:opacity-0 md:group-hover:opacity-100"
            onclick={(e) => {
              e.stopPropagation()
              menuOpenId = menuOpenId === project.id ? null : project.id
            }}
            aria-label="项目菜单"
          >
            <EllipsisVertical class="h-3.5 w-3.5" />
          </button>
        {/if}
      {/if}

      {#if menuOpenId === project.id}
        <div class="absolute top-full right-2 z-20 mt-0.5">
          <ContextMenu
            items={[
              {
                label: m.rename(),
                onclick: () => startEdit(project.id, project.name),
              },
              ...(authStore.role === 'owner'
                ? [
                    {
                      label: m.delete(),
                      onclick: () => deleteProject(project.id),
                      danger: true,
                    },
                  ]
                : []),
            ]}
            onclose={() => {
              menuOpenId = null
            }}
          />
        </div>
      {/if}

      {#if navigationStore.activeProjectId === project.id && foldersStore.foldersByProject[project.id]}
        <div class="pt-0.5 pl-3">
          <FolderTree
            folders={foldersStore.foldersByProject[project.id]}
            activeFolderId={navigationStore.activeFolderId}
            projectId={project.id}
            {selectedAssetIds}
            ingestToken={emailIngestToken}
            onselect={(folderId) => onFolderSelect(project.id, folderId)}
            onassetsDropped={(assetIds, folderId) =>
              onAssetsFolderDropped(assetIds, folderId, project.id)}
          />
          {#if authStore.role !== 'viewer'}
            {#if creatingFolderForProject === project.id}
              <form
                class="mt-1 flex items-center gap-1 rounded-md border border-[var(--border)] bg-[var(--bg-elevated)] px-2 py-1"
                onsubmit={(e) => {
                  e.preventDefault()
                  submitCreateFolder(project.id)
                }}
              >
                <input
                  bind:value={newFolderName}
                  placeholder="文件夹名称"
                  maxlength="100"
                  class="min-w-0 flex-1 bg-transparent text-sm text-[var(--text-primary)] outline-none"
                  onblur={() => {
                    if (!newFolderName.trim()) creatingFolderForProject = null
                  }}
                />
                <button
                  type="submit"
                  class="shrink-0 text-sm text-[var(--accent)] hover:text-[var(--accent-hover)]"
                  >{m.add()}</button
                >
              </form>
            {:else}
              <Button
                variant="ghost"
                size="sm"
                class="mt-0.5 w-full justify-start"
                onclick={() => {
                  creatingFolderForProject = project.id
                  newFolderName = ''
                }}
              >
                {#snippet icon()}<Plus class="h-3 w-3" />{/snippet}
                {m.new_folder()}
              </Button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/each}

  <!-- New project -->
  {#if authStore.role !== 'viewer'}
    {#if projectsStore.projects.length === 0 && !creating}
      <p class="px-2 py-2 text-xs leading-5 text-[var(--text-muted)]">
        还没有项目，可先建立一个项目再按文件夹归档素材。
      </p>
    {/if}
    {#if creating}
      <form
        class="mt-1 flex flex-col gap-2 rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-2"
        onsubmit={(e) => {
          e.preventDefault()
          submitCreate()
        }}
      >
        <input
          bind:value={newName}
          placeholder={m.project_name()}
          required
          maxlength="100"
          class="min-h-9 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] px-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-soft)] focus:outline-none"
        />
        <div class="flex gap-1">
          <Button type="submit" size="sm" class="flex-1">创建项目</Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            class="flex-1"
            onclick={() => onCreatingChange(false)}>{m.cancel()}</Button
          >
        </div>
      </form>
    {:else}
      <Button
        variant="ghost"
        size="sm"
        class="mt-1 w-full justify-start"
        onclick={() => onCreatingChange(true)}
      >
        {#snippet icon()}<Plus class="h-3.5 w-3.5" />{/snippet}
        {m.new_project()}
      </Button>
    {/if}
  {/if}

  <Feedback {error} />
</div>
