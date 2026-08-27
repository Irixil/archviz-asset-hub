<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { type Folder } from '$lib/api'
  import { ChevronRight, Dot, EllipsisVertical } from '@lucide/svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from './ui/Feedback.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { configStore } from '$lib/stores/config.svelte'
  import { m } from '$lib/paraglide/messages'
  import { getProjectColor } from '$lib/stores/assetView'
  import { projectsStore } from '$lib/stores/projects.svelte'

  interface Props {
    folders: Folder[]
    activeFolderId: string | null
    projectId: string
    selectedAssetIds: Set<string>
    ingestToken?: string | null
    onselect: (folderId: string | null) => void
    onassetsDropped: (assetIds: string[], folderId: string | null) => void
  }

  let {
    folders,
    activeFolderId,
    projectId,
    selectedAssetIds,
    ingestToken = null,
    onselect,
    onassetsDropped,
  }: Props = $props()

  let project = $derived(projectsStore.projects.find((p) => p.id === projectId))

  async function copyIngestAddress(folder: Folder) {
    if (!ingestToken || !folder.slug) return
    const address = `${ingestToken}+${folder.slug}@${configStore.state.mailHost}`
    try {
      await navigator.clipboard.writeText(address)
      toastStore.show(m.address_copied())
    } catch {
      toastStore.show(m.cannot_copy(), 'error')
    }
  }

  const STORAGE_KEY = 'folder-tree-open'

  function loadOpenIds(): Set<string> {
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (raw) return new Set(JSON.parse(raw))
    } catch {}
    return new Set()
  }

  function saveOpenIds(ids: Set<string>) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
    } catch {}
  }

  let openFolderIds = $state<Set<string>>(loadOpenIds())
  let contextMenuId = $state<string | null>(null)
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let creatingUnder = $state<string | null>(null)
  let newFolderName = $state('')
  let dropTargetId = $state<string | null>(null)
  let error = $state('')

  function toggleOpen(id: string, e: MouseEvent) {
    e.stopPropagation()
    const next = new Set(openFolderIds)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    openFolderIds = next
    saveOpenIds(next)
  }

  async function submitRename(id: string, name: string) {
    if (!name) {
      editingId = null
      return
    }
    try {
      await foldersStore.rename(id, projectId, name)
      editingId = null
    } catch {
      error = m.folder_rename_failed()
    }
  }

  async function deleteFolder(id: string) {
    contextMenuId = null
    try {
      await foldersStore.delete(id, projectId)
    } catch {
      error = m.folder_delete_failed()
    }
  }

  async function submitCreate(parentId: string | null) {
    const name = newFolderName.trim()
    if (!name) {
      creatingUnder = null
      return
    }
    try {
      const data: { name: string; parent_id?: string } = { name }
      if (parentId) data.parent_id = parentId
      await foldersStore.create(projectId, data)
      newFolderName = ''
      creatingUnder = null
    } catch {
      error = m.folder_create_failed()
    }
  }

  function handleDragOver(e: DragEvent, folderId: string | null) {
    e.preventDefault()
    dropTargetId = folderId
  }

  function handleDragLeave() {
    dropTargetId = null
  }

  function handleDrop(e: DragEvent, folderId: string | null) {
    e.preventDefault()
    dropTargetId = null
    const assetId = e.dataTransfer?.getData('text/plain')
    if (!assetId) return
    const assetIds =
      selectedAssetIds.has(assetId) && selectedAssetIds.size > 1
        ? [...selectedAssetIds]
        : [assetId]
    onassetsDropped(assetIds, folderId)
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      editingId = null
      contextMenuId = null
      creatingUnder = null
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#each folders as folder (folder.id)}
  <div class="mt-0.5">
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="group relative flex items-center rounded-md transition-colors
        {activeFolderId === folder.id ? 'bg-blue-50 dark:bg-blue-900/30' : ''}
        {dropTargetId === folder.id
        ? 'bg-blue-100 ring-1 ring-blue-400 dark:bg-blue-900/40'
        : ''}
      "
      style={activeFolderId === folder.id
        ? `background-color: ${getProjectColor(project, '22')};`
        : ''}
      ondragover={(e) => handleDragOver(e, folder.id)}
      ondragleave={handleDragLeave}
      ondrop={(e) => handleDrop(e, folder.id)}
    >
      {#if folder.children && folder.children.length > 0}
        <button
          class="shrink-0 p-1 text-gray-400 hover:text-gray-600"
          onclick={(e) => toggleOpen(folder.id, e)}
          aria-label="Toggle folder"
        >
          <ChevronRight
            class="h-4 w-4 transition-transform {openFolderIds.has(folder.id)
              ? 'rotate-90'
              : ''}"
            color={getProjectColor(project)}
          />
        </button>
      {:else}
        <Dot
          class="h-4 w-6 shrink-0"
          style="color: {getProjectColor(project)};"
        />
      {/if}

      {#if editingId === folder.id}
        <div class="flex-1 pr-1">
          <InlineEditForm
            bind:value={editName}
            onsubmit={(v) => submitRename(folder.id, v)}
            oncancel={() => {
              editingId = null
            }}
            size="sm"
          />
        </div>
      {:else}
        <button
          class="text-md flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-3 text-left {activeFolderId ===
          folder.id
            ? 'font-medium text-blue-700 dark:text-blue-400'
            : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
          onclick={() => onselect(folder.id)}
        >
          <span
            class="text-md min-w-0 flex-1 truncate"
            style={activeFolderId === folder.id
              ? `color: ${getProjectColor(project)}`
              : ''}>{folder.name}</span
          >
          {#if folder.asset_count > 0}
            <span class="text-md shrink-0 text-gray-400"
              >{folder.asset_count}</span
            >
          {/if}
        </button>
      {/if}

      {#if authStore.role !== 'viewer'}
        <button
          class="absolute top-1/2 right-7 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-100 hover:bg-gray-200 hover:text-gray-700 md:opacity-0 md:group-hover:opacity-100 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          onclick={(e) => {
            e.stopPropagation()
            contextMenuId = contextMenuId === folder.id ? null : folder.id
          }}
          aria-label="Folder menu"
        >
          <EllipsisVertical class="h-3.5 w-3.5" />
        </button>
      {/if}

      {#if contextMenuId === folder.id}
        <div class="absolute top-full right-0 z-30 mt-0.5">
          <ContextMenu
            items={[
              {
                label: m.rename(),
                onclick: () => {
                  editingId = folder.id
                  editName = folder.name
                  contextMenuId = null
                },
              },
              ...(folder.parent_id == null
                ? [
                    {
                      label: m.add_subfolder(),
                      onclick: () => {
                        creatingUnder = folder.id
                        contextMenuId = null
                        newFolderName = ''
                      },
                    },
                  ]
                : []),
              ...(ingestToken && folder.slug
                ? [
                    {
                      label: m.copy_address(),
                      onclick: () => {
                        copyIngestAddress(folder)
                        contextMenuId = null
                      },
                    },
                  ]
                : []),
              ...(authStore.role === 'owner'
                ? [
                    {
                      label: m.delete(),
                      onclick: () => deleteFolder(folder.id),
                      danger: true,
                    },
                  ]
                : []),
            ]}
            onclose={() => {
              contextMenuId = null
            }}
          />
        </div>
      {/if}
    </div>

    {#if creatingUnder === folder.id}
      <div class="ml-5">
        <form
          class="flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 dark:border-gray-700 dark:bg-gray-800"
          onsubmit={(e) => {
            e.preventDefault()
            submitCreate(folder.id)
          }}
        >
          <input
            bind:value={newFolderName}
            placeholder={m.name()}
            class="text-md min-w-0 flex-1 bg-transparent text-gray-900 outline-none dark:text-gray-100"
            onblur={() => {
              if (!newFolderName.trim()) creatingUnder = null
            }}
          />
          <Button type="submit" variant="ghost" size="sm" class="shrink-0"
            >{m.add()}</Button
          >
        </form>
      </div>
    {/if}

    {#if openFolderIds.has(folder.id) && folder.children && folder.children.length > 0}
      <div
        class="ml-4 border-l border-gray-100 pl-1 dark:border-gray-600"
        style="border-color: {getProjectColor(project)}"
      >
        {#each folder.children as child (child.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="group relative flex items-center rounded-md transition-colors
              {activeFolderId === child.id
              ? 'bg-blue-50 dark:bg-blue-900/30'
              : ''}
              {dropTargetId === child.id
              ? 'bg-blue-100 ring-1 ring-blue-400 dark:bg-blue-900/40'
              : ''}
            "
            ondragover={(e) => handleDragOver(e, child.id)}
            ondragleave={handleDragLeave}
            ondrop={(e) => handleDrop(e, child.id)}
          >
            <span class="w-3 shrink-0"></span>
            {#if editingId === child.id}
              <div class="flex-1 pr-1">
                <InlineEditForm
                  bind:value={editName}
                  onsubmit={(v) => submitRename(child.id, v)}
                  oncancel={() => {
                    editingId = null
                  }}
                  size="sm"
                />
              </div>
            {:else}
              <button
                class="flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-3.5 text-left {activeFolderId ===
                child.id
                  ? 'font-medium text-blue-700 dark:text-blue-400'
                  : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
                onclick={() => onselect(child.id)}
              >
                <span class="text-md min-w-0 flex-1 truncate">{child.name}</span
                >
                {#if child.asset_count > 0}
                  <span class="text-md shrink-0 text-gray-400"
                    >{child.asset_count}</span
                  >
                {/if}
              </button>
            {/if}

            {#if authStore.role !== 'viewer'}
              <button
                class="absolute top-1/2 right-7 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-100 hover:bg-gray-200 hover:text-gray-700 md:opacity-0 md:group-hover:opacity-100 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
                onclick={(e) => {
                  e.stopPropagation()
                  contextMenuId = contextMenuId === child.id ? null : child.id
                }}
                aria-label="Folder menu"
              >
                <EllipsisVertical class="h-3.5 w-3.5" />
              </button>
            {/if}

            {#if contextMenuId === child.id}
              <div class="absolute top-full right-0 z-30 mt-0.5">
                <ContextMenu
                  items={[
                    {
                      label: m.rename(),
                      onclick: () => {
                        editingId = child.id
                        editName = child.name
                        contextMenuId = null
                      },
                    },
                    ...(ingestToken && child.slug
                      ? [
                          {
                            label: m.copy_address(),
                            onclick: () => {
                              copyIngestAddress(child)
                              contextMenuId = null
                            },
                          },
                        ]
                      : []),
                    ...(authStore.role === 'owner'
                      ? [
                          {
                            label: m.delete(),
                            onclick: () => deleteFolder(child.id),
                            danger: true,
                          },
                        ]
                      : []),
                  ]}
                  onclose={() => {
                    contextMenuId = null
                  }}
                />
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/each}

<Feedback {error} />
