<script lang="ts">
  import { stackStore } from '$lib/stores/stack.svelte'
  import {
    apiFetch,
    stackApi,
    collectionApi,
    shareApi,
    projectApi,
    mimeCategory,
  } from '$lib/api'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/assetView'
  import { sseEvents } from '$lib/stores/assets.svelte'
  import type { Project } from '$lib/api'
  import {
    Layers,
    Trash2,
    Tag,
    Download,
    FolderInput,
    Save,
    Loader,
    Share2,
    LibraryBig,
  } from '@lucide/svelte'
  import type { Collection } from '$lib/api'
  import { fly } from 'svelte/transition'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import ButtonCancel from './ui/ButtonCancel.svelte'
  import Hint from './ui/Hint.svelte'
  import ButtonCopy from './ui/ButtonCopy.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { AddAssetsToCollection } from '$lib/commands/AddAssetsToCollection'

  let labelInput = $state(stackStore.label ?? '')
  $effect(() => {
    labelInput = stackStore.label ?? ''
  })
  let downloading = $state(false)

  // Save as collection
  let savingCollection = $state(false)
  let collectionName = $state('')
  let showCollectionForm = $state(false)

  // Share stack
  let sharing = $state(false)
  let shareUrl = $state<string | null>(null)
  let copied = $state(false)
  let showShareNameForm = $state(false)
  let shareNameInput = $state('')

  // Add to existing collection
  let showAddToCollection = $state(false)
  let existingCollections = $state<Collection[]>([])
  let loadingCollections = $state(false)
  let addingToCollection = $state<string | null>(null)

  async function handleOpenAddToCollection() {
    showAddToCollection = true
    loadingCollections = true
    try {
      existingCollections = await collectionApi.list()
    } catch {
      existingCollections = []
    } finally {
      loadingCollections = false
    }
  }

  async function handleAddToCollection(col: Collection) {
    addingToCollection = col.id
    try {
      await undoStore.execute(
        new AddAssetsToCollection(stackStore.ids, col.id, col.name)
      )
      showAddToCollection = false
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.save_failed(), 'error')
    } finally {
      addingToCollection = null
    }
  }

  // Move to project
  let movingToProject = $state(false)
  let showProjectPicker = $state(false)
  let projects = $state<Project[]>([])
  let projectSearch = $state('')
  let loadingProjects = $state(false)

  let filteredProjects = $derived(
    projects.filter((p) =>
      p.name.toLowerCase().includes(projectSearch.toLowerCase())
    )
  )

  // Merge
  let merging = $state(false)
  let mergeType = $state<'gif' | 'pdf'>('gif')
  let mergeStatus = $state<string | null>(null)
  let showMergeForm = $state(false)
  let pendingMergeJobId = $state<string | null>(null)
  let mergeTimeout: ReturnType<typeof setTimeout> | null = null

  $effect(() => {
    const ev = sseEvents.last
    if (
      ev?.type === 'stack_merge_done' &&
      ev.payload.job_id === pendingMergeJobId
    ) {
      if (mergeTimeout) {
        clearTimeout(mergeTimeout)
        mergeTimeout = null
      }
      pendingMergeJobId = null
      merging = false
      mergeStatus = null
      toastStore.show(m.merge_complete_asset_created(), 'success')
    }
  })

  $effect(() => {
    return () => {
      if (mergeTimeout) clearTimeout(mergeTimeout)
    }
  })

  function handleLabelBlur() {
    stackStore.setLabel(labelInput.trim())
  }

  async function handleDownloadZip() {
    downloading = true
    try {
      await stackApi.exportZip(
        stackStore.ids,
        stackStore.label ?? 'stack-export'
      )
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.download_failed(), 'error')
    } finally {
      downloading = false
    }
  }

  async function handleSaveCollection() {
    if (!collectionName.trim()) return
    savingCollection = true
    try {
      await collectionApi.create(collectionName.trim(), '', stackStore.ids)
      toastStore.show(m.collection_saved(), 'success')
      showCollectionForm = false
      collectionName = ''
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.save_failed(), 'error')
    } finally {
      savingCollection = false
    }
  }

  // Share: needs a saved collection first. If stack has a label, use it; otherwise prompt.
  function handleShareClick() {
    shareUrl = null
    copied = false
    if (stackStore.label?.trim()) {
      doShare(stackStore.label.trim())
    } else {
      shareNameInput = ''
      showShareNameForm = true
    }
  }

  async function doShare(name: string) {
    sharing = true
    showShareNameForm = false
    try {
      const col = await collectionApi.create(name, '', stackStore.ids)
      stackStore.setLabel(name)
      const share = await shareApi.create({
        target_type: 'collection',
        target_id: col.id,
        label: name,
        allow_download: true,
      })
      shareUrl = share.public_url
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.share_failed(), 'error')
    } finally {
      sharing = false
    }
  }

  async function handleCopyLink() {
    if (!shareUrl) return
    await navigator.clipboard.writeText(shareUrl)
    copied = true
    setTimeout(() => {
      copied = false
    }, 2000)
  }

  async function handleOpenProjectPicker() {
    showProjectPicker = true
    loadingProjects = true
    projectSearch = ''
    try {
      projects = await projectApi.list()
    } catch {
      projects = []
    } finally {
      loadingProjects = false
    }
  }

  async function handleMoveToProject(project: Project) {
    showProjectPicker = false
    movingToProject = true
    try {
      await apiFetch('/api/v1/assets/bulk/project', {
        method: 'POST',
        body: JSON.stringify({
          asset_ids: stackStore.ids,
          project_id: project.id,
        }),
      })
      toastStore.show(
        m.assets_moved_to({ count: stackStore.count, name: project.name }),
        'success'
      )
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.move_failed(), 'error')
    } finally {
      movingToProject = false
    }
  }

  async function handleMerge() {
    merging = true
    mergeStatus = 'pending'
    try {
      const jobId = await stackApi.merge(
        stackStore.ids,
        mergeType,
        stackStore.label ?? 'stack-merge'
      )
      showMergeForm = false
      pendingMergeJobId = jobId
      mergeTimeout = setTimeout(() => {
        if (pendingMergeJobId === jobId) {
          pendingMergeJobId = null
          merging = false
          mergeStatus = null
          toastStore.show(m.merge_timed_out(), 'error')
        }
      }, 30000)
    } catch (e: unknown) {
      toastStore.show((e as Error)?.message ?? m.merge_failed(), 'error')
      merging = false
      mergeStatus = null
    }
  }
</script>

<!-- Backdrop -->
<button
  type="button"
  class="fixed inset-0 z-40 bg-black/20 dark:bg-black/40"
  onclick={() => stackStore.closePanel()}
  aria-label={m.close()}
></button>

<!-- Panel -->
<div
  transition:fly={{ x: 400, duration: 250 }}
  class="fixed top-0 right-0 bottom-0 z-50 flex w-96 flex-col border-l border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900"
>
  <!-- Header -->
  <div
    class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800"
  >
    <div class="flex items-center gap-2">
      <Layers class="h-5 w-5 text-amber-500" />
      <span class="text-base font-semibold text-gray-900 dark:text-gray-100">
        {m.working_stack()}
        <span class="ml-1 text-sm font-normal text-gray-400"
          >({stackStore.count})</span
        >
      </span>
    </div>
    <ButtonCancel x onclick={() => stackStore.closePanel()} />
  </div>

  <!-- Label input -->
  <div class="border-b border-gray-100 px-5 py-3 dark:border-gray-800">
    <input
      type="text"
      placeholder={m.stack_label()}
      bind:value={labelInput}
      onblur={handleLabelBlur}
      class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-amber-400 focus:ring-1 focus:ring-amber-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
    />
  </div>

  <!-- Asset list -->
  <div class="flex-1 overflow-y-auto px-3 py-3">
    {#if stackStore.count === 0}
      <div
        class="flex flex-col items-center justify-center gap-2 py-16 text-gray-400"
      >
        <Layers class="h-10 w-10 opacity-30" />
        <Hint>{m.stack_is_empty()}</Hint>
      </div>
    {:else}
      <div class="flex flex-col gap-1">
        {#each stackStore.assets as asset (asset.id)}
          <div
            class="flex items-center gap-3 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-gray-800"
          >
            <div
              class="h-10 w-10 shrink-0 overflow-hidden rounded-md {ASSET_BACKGROUND_COLORS[
                mimeCategory(asset.mimeType)
              ] ?? 'bg-gray-100 dark:bg-gray-700'}"
            >
              {#if asset.thumbnailUrl}
                <img
                  src={asset.thumbnailUrl}
                  alt={asset.name}
                  class="h-full w-full object-cover"
                />
              {:else}
                <div class="flex h-full w-full items-center justify-center">
                  <Layers class="h-4 w-4 text-gray-400" />
                </div>
              {/if}
            </div>
            <p
              class="flex-1 truncate text-sm text-gray-800 dark:text-gray-200"
              title={asset.name}
            >
              {asset.name}
            </p>
            <ButtonCancel
              x
              onclick={() => stackStore.remove(asset.id)}
              title={m.remove_from_stack()}
            />
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Actions -->
  <div class="border-t border-gray-100 px-5 py-4 dark:border-gray-800">
    <div class="flex flex-col gap-2">
      <!-- Download ZIP -->
      <button
        type="button"
        disabled={downloading || stackStore.count === 0}
        onclick={handleDownloadZip}
        class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
          {downloading || stackStore.count === 0
          ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
          : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
      >
        {#if downloading}
          <Loader class="h-4 w-4 animate-spin" />
        {:else}
          <Download class="h-4 w-4" />
        {/if}
        {downloading ? m.downloading() : m.download_zip()}
      </button>

      <!-- Save as Collection -->
      {#if showCollectionForm}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950"
        >
          <input
            type="text"
            placeholder={m.collection_name()}
            bind:value={collectionName}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-amber-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          <div class="flex gap-2">
            <button
              type="button"
              disabled={savingCollection || !collectionName.trim()}
              onclick={handleSaveCollection}
              class="flex-1 rounded-lg bg-amber-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-600 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {savingCollection ? m.saving() : m.save()}
            </button>
            <ButtonCancel
              onclick={() => {
                showCollectionForm = false
                collectionName = ''
              }}
            />
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={stackStore.count === 0}
          onclick={() => (showCollectionForm = true)}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {stackStore.count === 0
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          <Save class="h-4 w-4" />
          {m.collection_save_as()}
        </button>
      {/if}

      <!-- Add to existing collection -->
      {#if showAddToCollection}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950"
        >
          {#if loadingCollections}
            <div class="flex justify-center py-2">
              <Loader class="h-4 w-4 animate-spin text-gray-400" />
            </div>
          {:else if existingCollections.length === 0}
            <p class="py-2 text-center text-xs text-gray-400">
              {m.no_collections()}
            </p>
          {:else}
            <div class="max-h-40 overflow-y-auto">
              {#each existingCollections as col (col.id)}
                <button
                  type="button"
                  disabled={addingToCollection === col.id}
                  onclick={() => handleAddToCollection(col)}
                  class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-gray-700 hover:bg-white disabled:opacity-50 dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  {#if addingToCollection === col.id}
                    <Loader
                      class="h-3.5 w-3.5 shrink-0 animate-spin text-gray-400"
                    />
                  {:else}
                    <LibraryBig class="h-3.5 w-3.5 shrink-0 text-gray-400" />
                  {/if}
                  <span class="flex-1 truncate text-left">{col.name}</span>
                  <span class="text-xs text-gray-400">{col.asset_count}</span>
                </button>
              {/each}
            </div>
          {/if}
          <ButtonCancel
            onclick={() => {
              showAddToCollection = false
            }}
          />
        </div>
      {:else}
        <button
          type="button"
          disabled={stackStore.count === 0}
          onclick={handleOpenAddToCollection}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {stackStore.count === 0
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          <LibraryBig class="h-4 w-4" />
          {m.add_to_collection()}
        </button>
      {/if}

      <!-- Share stack — WS-5 -->
      {#if shareUrl}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950"
        >
          <p class="text-xs font-medium text-green-700 dark:text-green-300">
            {m.share_link_ready()}
          </p>
          <div class="flex gap-2">
            <input
              type="text"
              readonly
              value={shareUrl}
              class="min-w-0 flex-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
            />
            <ButtonCopy {copied} onclick={handleCopyLink} />
          </div>
          <ButtonCancel
            onclick={() => {
              shareUrl = null
            }}>{m.dismiss()}</ButtonCancel
          >
        </div>
      {:else if showShareNameForm}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950"
        >
          <p class="text-xs font-medium text-green-700 dark:text-green-300">
            {m.collection_name_to_share()}
          </p>
          <input
            type="text"
            placeholder={m.collection_name()}
            bind:value={shareNameInput}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-green-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          <div class="flex gap-2">
            <button
              type="button"
              disabled={sharing || !shareNameInput.trim()}
              onclick={() => doShare(shareNameInput.trim())}
              class="flex-1 rounded-lg bg-green-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-green-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {sharing ? m.sharing() : m.share()}
            </button>
            <ButtonCancel
              onclick={() => {
                showShareNameForm = false
              }}
              class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
            />
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={sharing || stackStore.count === 0}
          onclick={handleShareClick}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {sharing || stackStore.count === 0
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if sharing}
            <Loader class="h-4 w-4 animate-spin" />
            {m.sharing()}
          {:else}
            <Share2 class="h-4 w-4" />
            {m.share_as_collection()}
          {/if}
        </button>
      {/if}

      <!-- Move to Project — WS-5 -->
      {#if showProjectPicker}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950"
        >
          <input
            type="text"
            placeholder="Search projects…"
            bind:value={projectSearch}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          {#if loadingProjects}
            <div class="flex justify-center py-2">
              <Loader class="h-4 w-4 animate-spin text-gray-400" />
            </div>
          {:else if filteredProjects.length === 0}
            <p class="py-2 text-center text-xs text-gray-400">
              {m.no_projects()}
            </p>
          {:else}
            <div class="max-h-40 overflow-y-auto">
              {#each filteredProjects as project (project.id)}
                <button
                  type="button"
                  onclick={() => handleMoveToProject(project)}
                  class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-gray-700 hover:bg-white dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  <FolderInput class="h-3.5 w-3.5 shrink-0 text-gray-400" />
                  {project.name}
                </button>
              {/each}
            </div>
          {/if}
          <ButtonCancel
            onclick={() => {
              showProjectPicker = false
            }}
          />
        </div>
      {:else}
        <button
          type="button"
          disabled={movingToProject || stackStore.count === 0}
          onclick={handleOpenProjectPicker}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {movingToProject || stackStore.count === 0
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if movingToProject}
            <Loader class="h-4 w-4 animate-spin" />
            {m.moving()}
          {:else}
            <FolderInput class="h-4 w-4" />
            {m.move_to_project()}
          {/if}
        </button>
      {/if}

      <!-- Merge as GIF / PDF -->
      {#if showMergeForm}
        <div
          class="flex flex-col gap-1.5 rounded-lg border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-800 dark:bg-indigo-950"
        >
          <div class="flex gap-2">
            <button
              type="button"
              onclick={() => (mergeType = 'gif')}
              class="flex-1 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors
                {mergeType === 'gif'
                ? 'border-indigo-500 bg-indigo-500 text-white'
                : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400'}"
              >GIF</button
            >
            <button
              type="button"
              onclick={() => (mergeType = 'pdf')}
              class="flex-1 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors
                {mergeType === 'pdf'
                ? 'border-indigo-500 bg-indigo-500 text-white'
                : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400'}"
              >PDF</button
            >
          </div>
          <div class="flex gap-2">
            <button
              type="button"
              disabled={merging}
              onclick={handleMerge}
              class="flex-1 rounded-lg bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-600 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {merging ? m.merging({ status: mergeStatus ?? '' }) : m.merge()}
            </button>
            <ButtonCancel onclick={() => (showMergeForm = false)} />
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={stackStore.count < 2}
          onclick={() => (showMergeForm = true)}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {stackStore.count < 2
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if merging}
            <Loader class="h-4 w-4 animate-spin" />
            {m.merging({ status: mergeStatus ?? '' })}
          {:else}
            <Tag class="h-4 w-4" />
            {m.merge_to_common()}
          {/if}
        </button>
      {/if}
    </div>

    <!-- Clear -->
    <button
      type="button"
      class="mt-4 flex w-full items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm text-red-500 hover:bg-red-50 dark:hover:bg-red-950"
      onclick={() => {
        stackStore.clear()
        stackStore.closePanel()
      }}
    >
      <Trash2 class="h-4 w-4" />
      {m.clear_all()}
    </button>
  </div>
</div>
