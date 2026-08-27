<script lang="ts">
  import { onMount } from 'svelte'
  import { formatDistanceToNowStrict } from 'date-fns'
  import { goto } from '$app/navigation'
  import {
    Plus,
    Trash2,
    AlertTriangle,
    ChevronDown,
    ChevronRight,
    ArrowUpDown,
    LayoutList,
    Layers,
    Check,
  } from '@lucide/svelte'
  import { tagsManagementStore } from '$lib/stores/tagsManagement.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { workspaceApi, ApiError } from '$lib/api'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { RenameTag } from '$lib/commands/RenameTag'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ButtonEdit from '$lib/components/ui/ButtonEdit.svelte'
  import ButtonCancel from '$lib/components/ui/ButtonCancel.svelte'
  import SystemTagBadge from '$lib/components/tags/SystemTagBadge.svelte'
  import type { Tag } from '$lib/api'
  import TagsBulkActionBar from '$lib/components/TagsBulkActionBar.svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { m } from '$lib/paraglide/messages'

  // ── View state ──────────────────────────────────────────────────────────────
  type SortKey = 'name' | 'asset_count' | 'last_used_at'
  type SortDir = 'asc' | 'desc'
  type ViewMode = 'flat' | 'grouped'

  let search = $state('')
  let sortKey = $state<SortKey>('asset_count')
  let sortDir = $state<SortDir>('desc')
  let viewMode = $state<ViewMode>('flat')
  let collapsedGroups = $state<Set<string>>(new Set())

  $effect(() => {
    if (tagsManagementStore.stale) tagsManagementStore.load()
  })

  // Restore preferences
  onMount(() => {
    const sk = localStorage.getItem('tags-sort-key')
    const sd = localStorage.getItem('tags-sort-dir')
    const vm = localStorage.getItem('tags-view-mode')
    const cg = localStorage.getItem('tags-collapsed-groups')
    if (sk) sortKey = sk as SortKey
    if (sd) sortDir = sd as SortDir
    if (vm) viewMode = vm as ViewMode
    if (cg) collapsedGroups = new Set(JSON.parse(cg))

    tagsManagementStore.load().then(() => {
      tagsManagementStore.loadDuplicates()
    })
  })

  $effect(() => {
    localStorage.setItem('tags-sort-key', sortKey)
    localStorage.setItem('tags-sort-dir', sortDir)
    localStorage.setItem('tags-view-mode', viewMode)
    localStorage.setItem(
      'tags-collapsed-groups',
      JSON.stringify([...collapsedGroups])
    )
  })

  // ── Derived list ────────────────────────────────────────────────────────────
  const filtered = $derived(
    tagsManagementStore.tags.filter((t) =>
      t.name.toLowerCase().includes(search.toLowerCase())
    )
  )

  const sorted = $derived(
    [...filtered].sort((a, b) => {
      let cmp = 0
      if (sortKey === 'name') cmp = a.name.localeCompare(b.name)
      else if (sortKey === 'asset_count') cmp = a.asset_count - b.asset_count
      else if (sortKey === 'last_used_at') {
        const ad = a.last_used_at ?? ''
        const bd = b.last_used_at ?? ''
        cmp = ad.localeCompare(bd)
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
  )

  function toggleSort(key: SortKey) {
    if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc'
    else {
      sortKey = key
      sortDir = 'desc'
    }
  }

  // ── Multi-select ─────────────────────────────────────────────────────────────
  let selected = $state<SvelteSet<string>>(new SvelteSet())

  function resetSelection(value?: Iterable<string> | null | undefined) {
    selected = new SvelteSet(value)
  }

  function toggleSelect(name: string) {
    const tag = tagsManagementStore.tags.find((t) => t.name === name)
    if (tag?.is_system) return
    if (selected.has(name)) selected.delete(name)
    else selected.add(name)
  }

  // ── Inline editing ───────────────────────────────────────────────────────────
  let editingName = $state<string | null>(null)
  let editingValue = $state('')
  let editNameError = $state('')

  function startEditName(name: string) {
    const tag = tagsManagementStore.tags.find((t) => t.name === name)
    if (tag?.is_system) {
      toastStore.show(m.tags_system_tag_rename_blocked(), 'error')
      return
    }
    editingName = name
    editingValue = name
    editNameError = ''
  }

  async function commitEditName(original: string) {
    const trimmed = editingValue.trim().toLowerCase()
    if (!trimmed || trimmed === original) {
      editingName = null
      return
    }
    try {
      await undoStore.execute(new RenameTag(original, trimmed))
    } catch (e: unknown) {
      if ((e as ApiError)?.status === 409)
        editNameError = m.tag_already_exists()
      else toastStore.show(m.tag_rename_failed(), 'error')
      return
    }
    editingName = null
  }

  async function handleColorChange(name: string, color: string) {
    try {
      await tagsManagementStore.patchTag(name, { color })
    } catch {
      toastStore.show(m.color_update_failed(), 'error')
    }
  }

  async function handleGroupChange(name: string, groupName: string | null) {
    try {
      await tagsManagementStore.patchTag(name, { group_name: groupName })
    } catch {
      toastStore.show(m.group_update_failed(), 'error')
    }
  }

  // ── Color picker ─────────────────────────────────────────────────────────────
  const PRESET_COLORS = [
    '#ef4444',
    '#f97316',
    '#eab308',
    '#22c55e',
    '#14b8a6',
    '#3b82f6',
    '#8b5cf6',
    '#ec4899',
    '#6b7280',
    '#1e293b',
    '#f1f5f9',
    '#ffffff',
  ]
  let colorPickerFor = $state<string | null>(null)

  // ── Group picker ─────────────────────────────────────────────────────────────
  let groupPickerFor = $state<string | null>(null)
  let groupPickerValue = $state('')
  let groupPickerAdding = $state(false)

  function openGroupPicker(name: string, current: string | null | undefined) {
    groupPickerFor = name
    groupPickerValue = current ?? ''
    groupPickerAdding = false
  }

  // ── New tag modal ─────────────────────────────────────────────────────────────
  let showNewTagModal = $state(false)
  let newTagName = $state('')
  let newTagColor = $state<string | null>(null)
  let newTagGroup = $state<string | null>(null)
  let newTagError = $state('')
  let newTagSaving = $state(false)

  async function submitNewTag() {
    newTagError = ''
    const name = newTagName.trim().toLowerCase()
    if (!name) {
      newTagError = m.name_required()
      return
    }
    newTagSaving = true
    try {
      await tagsManagementStore.createTag(name, newTagColor, newTagGroup)
      toastStore.show('Tag created')
      showNewTagModal = false
      newTagName = ''
      newTagColor = null
      newTagGroup = null
    } catch (e: unknown) {
      newTagError =
        (e as ApiError)?.status === 409
          ? m.tag_already_exists()
          : m.tag_create_failed()
    } finally {
      newTagSaving = false
    }
  }

  // ── Purge unused ─────────────────────────────────────────────────────────────
  let showPurgeModal = $state(false)
  const unusedTags = $derived(
    tagsManagementStore.tags.filter((t) => t.asset_count === 0 && !t.is_system)
  )

  async function purgeUnused() {
    try {
      const res = await tagsManagementStore.bulkDelete(
        unusedTags.map((t) => t.name)
      )
      toastStore.show(
        `Deleted ${res.deleted} unused tag${res.deleted !== 1 ? 's' : ''}`
      )
      showPurgeModal = false
    } catch {
      toastStore.show(m.purge_tags_failed(), 'error')
    }
  }

  // ── Bulk delete selected ──────────────────────────────────────────────────────
  let showBulkDeleteModal = $state(false)

  async function bulkDeleteSelected() {
    const names = [...selected].filter((name) => {
      const tag = tagsManagementStore.tags.find((t) => t.name === name)
      return !tag?.is_system
    })
    try {
      const res = await tagsManagementStore.bulkDelete(names)
      toastStore.show(
        `Deleted ${res.deleted} tag${res.deleted !== 1 ? 's' : ''}`
      )
      resetSelection()
      showBulkDeleteModal = false
    } catch {
      toastStore.show(m.delete_tags_failed(), 'error')
    }
  }

  // ── Merge modal ───────────────────────────────────────────────────────────────
  let showMergeModal = $state(false)
  let mergeTarget = $state<string | null>(null)
  let mergeSources = $state<string[]>([])
  let mergeSaving = $state(false)

  // Drag-to-merge
  let draggingTag = $state<string | null>(null)
  let dragOverTag = $state<string | null>(null)

  function openMergeForSelected() {
    const names = [...selected]
    mergeSources = names
    mergeTarget = names[0]
    showMergeModal = true
  }

  function openMergeForPair(a: string, b: string) {
    // Default target = higher asset_count
    const ta = tagsManagementStore.tags.find((t) => t.name === a)
    const tb = tagsManagementStore.tags.find((t) => t.name === b)
    const target = (ta?.asset_count ?? 0) >= (tb?.asset_count ?? 0) ? a : b
    const source = target === a ? b : a
    mergeSources = [source, target]
    mergeTarget = target
    showMergeModal = true
  }

  async function commitMerge() {
    if (!mergeTarget) return
    const sources = mergeSources.filter((s) => s !== mergeTarget)
    if (!sources.length) return
    mergeSaving = true
    try {
      const res = await tagsManagementStore.mergeTags(sources, mergeTarget)
      toastStore.show(
        `Merged ${res.merged_assets} asset${res.merged_assets !== 1 ? 's' : ''}`
      )
      resetSelection()
      showMergeModal = false
    } catch {
      toastStore.show(m.tag_merge_failed(), 'error')
    } finally {
      mergeSaving = false
    }
  }

  function handleDragStart(name: string) {
    draggingTag = name
  }
  function handleDragOver(e: DragEvent, name: string) {
    e.preventDefault()
    dragOverTag = name
  }
  function handleDrop(targetName: string) {
    if (!draggingTag || draggingTag === targetName) {
      draggingTag = null
      dragOverTag = null
      return
    }
    openMergeForPair(draggingTag, targetName)
    draggingTag = null
    dragOverTag = null
  }
  function handleDragEnd() {
    draggingTag = null
    dragOverTag = null
  }

  // ── Duplicate suggestions panel ────────────────────────────────────────────
  let showDupPanel = $state(false)

  // ── Group view helpers ─────────────────────────────────────────────────────
  const groupedTags = $derived(() => {
    const groups = new Map<string, typeof sorted>()
    for (const tag of sorted) {
      const g = tag.group_name ?? '__ungrouped__'
      if (!groups.has(g)) groups.set(g, [])
      groups.get(g)!.push(tag)
    }
    // Sort groups: named groups first alphabetically, then ungrouped
    const entries = [...groups.entries()].sort(([a], [b]) => {
      if (a === '__ungrouped__') return 1
      if (b === '__ungrouped__') return -1
      return a.localeCompare(b)
    })
    return entries
  })

  function toggleGroup(group: string) {
    const s = new Set(collapsedGroups)
    if (s.has(group)) s.delete(group)
    else s.add(group)
    collapsedGroups = s
  }

  // ── Relative time ────────────────────────────────────────────────────────────
  function relTime(iso: string | null | undefined): string {
    if (!iso) return '—'
    return formatDistanceToNowStrict(new Date(iso), { addSuffix: true })
  }

  // ── Locked vocabulary toggle (owner only) ─────────────────────────────────────
  const isOwner = $derived(authStore.role === 'owner')
  let taxonomyLocked = $state(authStore.workspace?.locked_taxonomy ?? false)
  let taxonomySaving = $state(false)

  $effect(() => {
    taxonomyLocked = authStore.workspace?.locked_taxonomy ?? false
  })

  async function toggleLockedTaxonomy() {
    if (!isOwner) return
    const next = !taxonomyLocked
    taxonomySaving = true
    try {
      const updated = await workspaceApi.updateSettings({
        locked_taxonomy: next,
      })
      authStore.patchWorkspace({ locked_taxonomy: updated.locked_taxonomy })
      taxonomyLocked = updated.locked_taxonomy
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : '更新设置失败', 'error')
    } finally {
      taxonomySaving = false
    }
  }

  // ── AI auto-tagging settings (owner only) ─────────────────────────────────────
  let autoTagEnabled = $state(authStore.workspace?.auto_tag_enabled ?? false)
  let autoTagMode = $state<'pending' | 'silent'>(
    (authStore.workspace?.auto_tag_mode as 'pending' | 'silent') ?? 'pending'
  )
  let autoTagSaving = $state(false)

  $effect(() => {
    autoTagEnabled = authStore.workspace?.auto_tag_enabled ?? false
    autoTagMode =
      (authStore.workspace?.auto_tag_mode as 'pending' | 'silent') ?? 'pending'
  })

  async function saveAutoTagSettings() {
    if (!isOwner) return
    autoTagSaving = true
    try {
      const updated = await workspaceApi.updateSettings({
        auto_tag_enabled: autoTagEnabled,
        auto_tag_mode: autoTagMode,
      })
      authStore.patchWorkspace({
        auto_tag_enabled: updated.auto_tag_enabled,
        auto_tag_mode: updated.auto_tag_mode,
      })
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : '更新设置失败', 'error')
    } finally {
      autoTagSaving = false
    }
  }

  async function toggleAutoTagEnabled() {
    autoTagEnabled = !autoTagEnabled
    await saveAutoTagSettings()
  }
</script>

<svelte:head>
  <title>{m.tags()}｜建筑素材中台</title>
</svelte:head>

<!-- Close popovers on outside click -->
<svelte:window
  onclick={(e) => {
    const t = e.target as HTMLElement
    if (!t.closest('[data-color-picker]')) colorPickerFor = null
    if (!t.closest('[data-group-picker]')) {
      groupPickerFor = null
      groupPickerAdding = false
    }
  }}
/>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader title={m.tags()} description={m.tags_page_description()}>
    <div class="space-x-2">
      {#if unusedTags.length > 0}
        <Button variant="primary" onclick={() => (showPurgeModal = true)}>
          {#snippet icon()}<Trash2 class="h-3.5 w-3.5" />{/snippet}
          {m.tags_purge_title()} ({unusedTags.length})
        </Button>
      {/if}
      {#if authStore.canCreateTag}
        <Button
          variant="primary"
          onclick={() => {
            showNewTagModal = true
            newTagName = ''
            newTagColor = null
            newTagGroup = null
            newTagError = ''
          }}
        >
          {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
          {m.tag_new()}
        </Button>
      {/if}
    </div>
  </PageHeader>

  <div class="flex-1 overflow-y-auto px-6 py-6">
    <div class="mx-auto w-full max-w-4xl">
      <!-- Tag taxonomy section — owner only -->
      {#if isOwner}
        <div class="mb-8">
          <div
            class="space-y-4 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
          >
            <h2
              class="text-sm font-semibold tracking-wide text-[var(--text-muted)] uppercase"
            >
              {m.workspace_settings_tag_taxonomy_section()}
            </h2>
            <div class="flex items-start justify-between gap-4">
              <div class="flex-1">
                <p class="text-md font-medium text-[var(--text-primary)]">
                  {m.workspace_settings_locked_taxonomy_label()}
                </p>
                <p class="mt-0.5 text-sm text-[var(--text-muted)]">
                  {m.workspace_settings_locked_taxonomy_description()}
                </p>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={taxonomyLocked}
                aria-label={m.workspace_settings_locked_taxonomy_label()}
                disabled={taxonomySaving}
                onclick={toggleLockedTaxonomy}
                class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-300 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus-visible:ring-indigo-500
              {taxonomyLocked
                  ? 'bg-indigo-600 hover:bg-indigo-700'
                  : 'bg-[var(--border)] hover:bg-[var(--border-subtle)] dark:hover:bg-[var(--bg-hover)]'}"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
              {taxonomyLocked ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>
            {#if taxonomyLocked}
              <p class="text-sm text-indigo-600 dark:text-indigo-400">
                {m.workspace_settings_locked_taxonomy_confirm({
                  count: tagsManagementStore.tags.filter((t) => !t.is_system)
                    .length,
                })}
              </p>
            {/if}
          </div>
        </div>
      {/if}

      <!-- Toolbar -->
      <div
        class="flex items-center gap-3 border-b border-gray-100 py-3 dark:border-gray-800"
      >
        <div class="flex-1">
          <SearchInput bind:value={search} placeholder={m.tags_search()} />
        </div>

        <!-- Duplicate banner -->
        {#if tagsManagementStore.duplicates.length > 0}
          <button
            class="flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-1.5 text-sm text-amber-700 hover:bg-amber-100 dark:bg-amber-900/20 dark:text-amber-400 dark:hover:bg-amber-900/30"
            onclick={() => (showDupPanel = true)}
          >
            找到 {tagsManagementStore.duplicates.length} 组疑似重复标签
            <ChevronRight class="h-3.5 w-3.5" />
          </button>
        {/if}

        <!-- View toggle -->
        <div
          class="flex rounded-lg border border-gray-200 dark:border-gray-700"
        >
          <button
            class="rounded-l-lg px-2.5 py-1.5 text-sm transition-colors
            {viewMode === 'flat'
              ? 'bg-gray-100 text-gray-900 dark:bg-gray-700 dark:text-gray-100'
              : 'text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => (viewMode = 'flat')}
            title="平铺列表"
          >
            <LayoutList class="h-4 w-4" />
          </button>
          <button
            class="rounded-r-lg px-2.5 py-1.5 text-sm transition-colors
            {viewMode === 'grouped'
              ? 'bg-gray-100 text-gray-900 dark:bg-gray-700 dark:text-gray-100'
              : 'text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => (viewMode = 'grouped')}
            title="按分组查看"
          >
            <Layers class="h-4 w-4" />
          </button>
        </div>
      </div>

      <!-- Main content -->
      <div
        class="mt-3 overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-gray-700 dark:bg-gray-900"
      >
        <div class="px-6 py-4">
          {#if tagsManagementStore.loading}
            <div class="flex justify-center py-16"><Spinner size="md" /></div>
          {:else if sorted.length === 0}
            <EmptyState
              title={search ? '没有匹配的标签' : '暂无标签'}
              description={search
                ? 'Try a different search term.'
                : '为素材添加标签时会自动创建，也可以点击“新建标签”手动创建。'}
            />
          {:else}
            <!-- Column headers (flat view) -->
            {#if viewMode === 'flat'}
              <div
                class="mb-1 grid grid-cols-[24px_16px_1fr_160px_80px_100px_80px] items-center gap-3 px-3 py-1.5 text-xs font-medium tracking-wide text-gray-400 dark:text-gray-500"
              >
                <span></span>
                <span></span>
                <button
                  class="flex items-center gap-1 text-left hover:text-gray-600 dark:hover:text-gray-300"
                  onclick={() => toggleSort('name')}
                >
                  {m.name()}
                  {#if sortKey === 'name'}<ArrowUpDown class="h-3 w-3" />{/if}
                </button>
                <span>{m.group()}</span>
                <button
                  class="flex items-center gap-1 text-right hover:text-gray-600 dark:hover:text-gray-300"
                  onclick={() => toggleSort('asset_count')}
                >
                  {m.assets()}
                  {#if sortKey === 'asset_count'}<ArrowUpDown
                      class="h-3 w-3"
                    />{/if}
                </button>
                <button
                  class="flex items-center gap-1 hover:text-gray-600 dark:hover:text-gray-300"
                  onclick={() => toggleSort('last_used_at')}
                >
                  {m.last_used()}
                  {#if sortKey === 'last_used_at'}<ArrowUpDown
                      class="h-3 w-3"
                    />{/if}
                </button>
                <span></span>
              </div>
            {/if}

            {#if viewMode === 'flat'}
              {#each sorted as tag (tag.id)}
                {@render tagRow(tag)}
              {/each}
            {:else}
              {#each groupedTags() as [group, groupTags] (group)}
                {@const label = group === '__ungrouped__' ? '未分组' : group}
                {@const totalCount = groupTags.reduce(
                  (s, t) => s + t.asset_count,
                  0
                )}
                {@const collapsed = collapsedGroups.has(group)}
                <div class="mb-4">
                  <button
                    class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-semibold text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                    onclick={() => toggleGroup(group)}
                  >
                    {#if collapsed}
                      <ChevronRight class="h-4 w-4 text-gray-400" />
                    {:else}
                      <ChevronDown class="h-4 w-4 text-gray-400" />
                    {/if}
                    <span>{label}</span>
                    <span class="ml-auto text-xs text-gray-400"
                      >{totalCount} 个素材</span
                    >
                  </button>
                  {#if !collapsed}
                    <div class="mt-1 space-y-0.5">
                      {#each groupTags as tag (tag.id)}
                        {@render tagRow(tag)}
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            {/if}
          {/if}
        </div>

        <!-- Duplicate suggestions slide-in panel -->
        {#if showDupPanel}
          <div
            class="w-80 shrink-0 overflow-y-auto border-l border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
          >
            <div
              class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-gray-800"
            >
              <span
                class="text-sm font-semibold text-gray-900 dark:text-gray-100"
                >{m.tags_possible_duplicates()}</span
              >
              <ButtonCancel
                x
                title="关闭"
                onclick={() => (showDupPanel = false)}
              />
            </div>
            <div class="divide-y divide-gray-100 dark:divide-gray-800">
              {#each tagsManagementStore.duplicates as pair (pair.a + pair.b)}
                <div class="flex items-center gap-2 px-4 py-3">
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5 text-sm">
                      <span
                        class="truncate font-medium text-gray-900 dark:text-gray-100"
                        >{pair.a}</span
                      >
                      <span class="text-gray-400">↔</span>
                      <span
                        class="truncate font-medium text-gray-900 dark:text-gray-100"
                        >{pair.b}</span
                      >
                    </div>
                    <div class="mt-0.5 text-xs text-gray-400">
                      {Math.round(pair.score * 100)}% different
                    </div>
                  </div>
                  <ButtonEdit
                    title={m.merge()}
                    class="text-xs font-medium text-indigo-600 hover:bg-indigo-50 dark:text-indigo-400 dark:hover:bg-indigo-900/30"
                    onclick={() => openMergeForPair(pair.a, pair.b)}
                  >
                    {m.merge()}
                  </ButtonEdit>
                  <ButtonCancel
                    x
                    title={m.dismiss()}
                    onclick={() =>
                      tagsManagementStore.dismissDuplicate(pair.a, pair.b)}
                  />
                </div>
              {:else}
                <div class="px-4 py-8 text-center text-sm text-gray-400">
                  {m.tags_duplicates_resolved()}
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<TagsBulkActionBar
  {selected}
  ondelete={bulkDeleteSelected}
  onmerge={openMergeForSelected}
  onclear={() => resetSelection()}
/>

<!-- ── Tag row snippet ─────────────────────────────────────────────────────── -->
{#snippet tagRow(tag: Tag)}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="group grid grid-cols-[24px_16px_1fr_160px_80px_100px_80px] items-center gap-3 rounded-lg px-3 py-2 transition-colors
      {tag.asset_count === 0
      ? 'bg-gray-50/60 dark:bg-gray-800/30'
      : 'hover:bg-gray-50 dark:hover:bg-gray-800/40'}
      {dragOverTag === tag.name ? 'ring-2 ring-indigo-400' : ''}"
    draggable="true"
    ondragstart={() => handleDragStart(tag.name)}
    ondragover={(e) => handleDragOver(e, tag.name)}
    ondrop={() => handleDrop(tag.name)}
    ondragend={handleDragEnd}
  >
    <!-- Checkbox -->
    {#if tag.is_system}
      <span></span>
    {:else}
      <input
        type="checkbox"
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 dark:border-gray-600"
        checked={selected.has(tag.name)}
        onchange={() => toggleSelect(tag.name)}
      />
    {/if}

    <!-- Color swatch -->
    <div class="relative flex items-center" data-color-picker>
      <button
        class="h-4 w-4 rounded-full border border-gray-200 transition-transform hover:scale-120 dark:border-gray-700"
        style="background-color: {tag.color ?? '#e5e7eb'}"
        onclick={(e) => {
          e.stopPropagation()
          colorPickerFor = colorPickerFor === tag.name ? null : tag.name
        }}
        title={m.change_color()}
      ></button>

      {#if colorPickerFor === tag.name}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="absolute top-6 left-0 z-30 w-48 rounded-xl border border-gray-200 bg-white p-3 shadow-lg dark:border-gray-700 dark:bg-gray-900"
          onclick={(e) => e.stopPropagation()}
        >
          <div class="mb-2 grid grid-cols-6 gap-1.5">
            {#each PRESET_COLORS as c}
              <button
                aria-label="Select color {c}"
                class="h-6 w-6 rounded-full border-2 transition-transform hover:scale-120
                  {tag.color === c
                  ? 'border-gray-900 dark:border-gray-100'
                  : 'border-transparent'}"
                style="background-color: {c}"
                onclick={() => {
                  handleColorChange(tag.name, c)
                  colorPickerFor = null
                }}
              ></button>
            {/each}
          </div>
          <input
            type="color"
            class="h-7 w-full cursor-pointer rounded border border-gray-200 dark:border-gray-700"
            value={tag.color ?? '#6b7280'}
            onchange={(e) => {
              handleColorChange(tag.name, e.currentTarget.value)
              colorPickerFor = null
            }}
          />
        </div>
      {/if}
    </div>

    <!-- Name (inline edit) -->
    <div class="min-w-0">
      {#if editingName === tag.name}
        <div class="flex items-center gap-1">
          <input
            class="w-full rounded border border-indigo-400 bg-white px-2 py-0.5 text-sm text-gray-900 outline-none focus:ring-1 focus:ring-indigo-500 dark:bg-gray-800 dark:text-gray-100
              {editNameError ? 'border-red-400' : ''}"
            bind:value={editingValue}
            onkeydown={(e) => {
              if (e.key === 'Enter') commitEditName(tag.name)
              if (e.key === 'Escape') editingName = null
            }}
            onblur={() => commitEditName(tag.name)}
          />
          <Feedback error={editNameError} />
        </div>
      {:else}
        <button
          class="truncate text-left text-sm font-medium dark:text-gray-100 {tag.is_system
            ? 'cursor-default text-gray-500 hover:text-gray-500 dark:text-gray-400 dark:hover:text-gray-400'
            : 'text-gray-900 hover:text-indigo-600 dark:hover:text-indigo-400'}"
          onclick={() => startEditName(tag.name)}
          title={tag.is_system
            ? m.tags_system_tag_rename_blocked()
            : m.rename()}
        >
          {tag.name}
        </button>
      {/if}
    </div>

    <!-- Group inline -->
    <div class="relative" data-group-picker>
      {#if tag.is_system}
        <SystemTagBadge />
      {:else if groupPickerFor === tag.name}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="absolute top-6 left-0 z-30 w-48 rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900"
          onclick={(e) => e.stopPropagation()}
        >
          {#each tagsManagementStore.allGroups as g}
            <button
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-800
                {tag.group_name === g
                ? 'font-medium text-indigo-600 dark:text-indigo-400'
                : 'text-gray-700 dark:text-gray-300'}"
              onclick={() => {
                handleGroupChange(tag.name, g)
                groupPickerFor = null
              }}
            >
              {#if tag.group_name === g}<Check
                  class="h-3.5 w-3.5 shrink-0"
                />{:else}<span class="h-3.5 w-3.5"></span>{/if}
              {g}
            </button>
          {/each}
          {#if tag.group_name}
            <ButtonCancel
              x
              title="Remove group"
              onclick={() => {
                handleGroupChange(tag.name, null)
                groupPickerFor = null
              }}
            />
          {/if}
          {#if !groupPickerAdding}
            <button
              class="flex w-full items-center gap-2 border-t border-gray-100 px-3 py-2 text-left text-sm text-gray-400 hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800"
              onclick={() => {
                groupPickerAdding = true
                groupPickerValue = ''
              }}
            >
              <Plus class="h-3.5 w-3.5" />
              {m.add_group()}
            </button>
          {:else}
            <div
              class="border-t border-gray-100 px-3 py-2 dark:border-gray-800"
            >
              <input
                class="w-full rounded border border-gray-200 px-2 py-1 text-sm outline-none focus:border-indigo-400 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
                bind:value={groupPickerValue}
                placeholder={m.name()}
                onkeydown={(e) => {
                  if (e.key === 'Enter' && groupPickerValue.trim()) {
                    handleGroupChange(tag.name, groupPickerValue.trim())
                    groupPickerFor = null
                    groupPickerAdding = false
                  }
                  if (e.key === 'Escape') groupPickerAdding = false
                }}
              />
            </div>
          {/if}
        </div>
      {:else}
        <button
          class="truncate text-left text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          onclick={(e) => {
            e.stopPropagation()
            openGroupPicker(tag.name, tag.group_name)
          }}
        >
          {tag.group_name ?? '—'}
        </button>
      {/if}
    </div>

    <!-- Asset count -->
    <div class="text-right">
      <button
        class="rounded px-1.5 py-0.5 text-sm text-gray-600 tabular-nums hover:bg-gray-100 hover:text-indigo-600 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-indigo-400"
        onclick={() => goto(`/library?tag=${encodeURIComponent(tag.name)}`)}
        title={m.open_in_library()}
      >
        {tag.asset_count}
      </button>
    </div>

    <!-- Last used -->
    <span class="text-sm text-gray-400 dark:text-gray-500"
      >{relTime(tag.last_used_at)}</span
    >

    <!-- Actions -->
    <div
      class="flex items-center justify-end gap-0.5 opacity-0 transition-opacity group-hover:opacity-100"
    >
      {#if !tag.is_system}
        <ButtonEdit
          onclick={() => startEditName(tag.name)}
          title={m.rename()}
        />
        <ButtonDelete
          onclick={async () => {
            try {
              await tagsManagementStore.bulkDelete([tag.name])
              toastStore.show(m.tag_deleted())
            } catch {
              toastStore.show(m.tag_delete_failed(), 'error')
            }
          }}
        />
      {/if}
    </div>
  </div>
{/snippet}

<!-- ── New tag modal ──────────────────────────────────────────────────────── -->
<Modal bind:open={showNewTagModal} onclose={() => (showNewTagModal = false)}>
  <div class="w-full rounded-2xl bg-white p-6 dark:bg-gray-900">
    <h2 class="mb-4 text-base font-semibold text-gray-900 dark:text-gray-100">
      New tag
    </h2>
    <div class="space-y-4">
      <div>
        <label
          for="new-tag-name"
          class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
          >{m.name()} <span class="text-red-500">*</span></label
        >
        <input
          id="new-tag-name"
          class="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm outline-none focus:border-indigo-400 focus:ring-1 focus:ring-indigo-400 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100
            {newTagError ? 'border-red-400' : ''}"
          bind:value={newTagName}
          placeholder="tag-name"
          onkeydown={(e) => e.key === 'Enter' && submitNewTag()}
        />
        <Feedback error={newTagError} />
      </div>

      <div>
        <label
          for="new-tag-color"
          class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
          >Color</label
        >
        <div class="flex flex-wrap gap-2">
          {#each PRESET_COLORS as c}
            <button
              aria-label="Select color {c}"
              class="h-6 w-6 rounded-full border-2 transition-transform hover:scale-120
                {newTagColor === c
                ? 'border-gray-900 dark:border-gray-100'
                : 'border-transparent'}"
              style="background-color: {c}"
              onclick={() => (newTagColor = newTagColor === c ? null : c)}
            ></button>
          {/each}
          <input
            type="color"
            class="h-6 w-6 cursor-pointer rounded border border-gray-200 dark:border-gray-700"
            value={newTagColor ?? '#6b7280'}
            onchange={(e) => (newTagColor = e.currentTarget.value)}
          />
        </div>
      </div>

      <div>
        <label
          for="new-tag-group"
          class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
          >Group</label
        >
        <select
          id="new-tag-group"
          class="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm outline-none focus:border-indigo-400 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
          value={newTagGroup ?? ''}
          onchange={(e) => (newTagGroup = e.currentTarget.value || null)}
        >
          <option value="">— 无 —</option>
          {#each tagsManagementStore.allGroups as g}
            <option value={g}>{g}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="mt-6 flex justify-end gap-2">
      <button
        class="rounded-lg px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
        onclick={() => (showNewTagModal = false)}>{m.cancel()}</button
      >
      <button
        class="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-600"
        onclick={submitNewTag}
        disabled={newTagSaving}
      >
        {#if newTagSaving}<Spinner size="sm" />{/if}
        {m.create()}
      </button>
    </div>
  </div>
</Modal>

<!-- ── Purge unused modal ─────────────────────────────────────────────────── -->
<Modal bind:open={showPurgeModal} onclose={() => (showPurgeModal = false)}>
  <div class="w-full rounded-2xl bg-white p-6 dark:bg-gray-900">
    <div class="mb-4 flex items-start gap-3">
      <AlertTriangle class="mt-0.5 h-5 w-5 shrink-0 text-amber-500" />
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
          {m.tags_purge_title()}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          以下 {unusedTags.length} 个标签将被永久删除：
        </p>
      </div>
    </div>
    <div
      class="mb-4 max-h-48 overflow-y-auto rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-gray-800 dark:bg-gray-800/40"
    >
      {#each unusedTags as t}
        <div class="py-0.5 text-sm text-gray-700 dark:text-gray-300">
          {t.name}
        </div>
      {/each}
    </div>
    <div class="flex justify-end gap-2">
      <button
        class="rounded-lg px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
        onclick={() => (showPurgeModal = false)}>{m.cancel()}</button
      >
      <button
        class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 dark:bg-red-700 dark:hover:bg-red-600"
        onclick={purgeUnused}>删除 {unusedTags.length} 个标签</button
      >
    </div>
  </div>
</Modal>

<!-- ── Bulk delete modal ──────────────────────────────────────────────────── -->
<Modal
  bind:open={showBulkDeleteModal}
  onclose={() => (showBulkDeleteModal = false)}
>
  <div class="w-full rounded-2xl bg-white p-6 dark:bg-gray-900">
    <h2 class="mb-3 text-base font-semibold text-gray-900 dark:text-gray-100">
      删除选中的 {selected.size} 个标签？
    </h2>
    <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
      {m.tags_bulk_delete_description()}
    </p>
    <div class="flex justify-end gap-2">
      <button
        class="rounded-lg px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
        onclick={() => (showBulkDeleteModal = false)}>{m.cancel()}</button
      >
      <button
        class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 dark:bg-red-700 dark:hover:bg-red-600"
        onclick={bulkDeleteSelected}>{m.delete()}</button
      >
    </div>
  </div>
</Modal>

<!-- ── Merge modal ────────────────────────────────────────────────────────── -->
<Modal
  bind:open={showMergeModal}
  onclose={() => {
    showMergeModal = false
  }}
>
  <div class="w-full rounded-2xl bg-white p-6 dark:bg-gray-900">
    <h2 class="mb-1 text-base font-semibold text-gray-900 dark:text-gray-100">
      {m.tags_merge_title()}
    </h2>
    <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
      {m.tags_merge_description()}
    </p>

    <div class="mb-4 space-y-1">
      {#each mergeSources as name}
        {@const tag = tagsManagementStore.tags.find((t) => t.name === name)}
        <label
          class="flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 transition-colors
          {mergeTarget === name
            ? 'border-indigo-400 bg-indigo-50 dark:border-indigo-600 dark:bg-indigo-950/30'
            : 'border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800'}"
        >
          <input
            type="radio"
            class="text-indigo-600"
            bind:group={mergeTarget}
            value={name}
          />
          <span
            class="flex-1 text-sm font-medium text-gray-900 dark:text-gray-100"
            >{name}</span
          >
          <span class="text-xs text-gray-400"
            >{tag?.asset_count ?? 0} assets</span
          >
        </label>
      {/each}
    </div>

    {#if mergeTarget}
      <p
        class="mb-4 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {m.tags_will_be_deleted({
          ids: mergeSources
            .filter((s) => s !== mergeTarget)
            .map((t) => `"${t}"`)
            .join(', '),
        })}.
      </p>
    {/if}

    <div class="flex justify-end gap-2">
      <ButtonCancel
        onclick={() => {
          showMergeModal = false
        }}
      />

      <ButtonEdit
        class="bg-indigo-600 px-4 py-2 font-medium text-white hover:bg-indigo-700 hover:text-white disabled:opacity-50 dark:bg-indigo-500 dark:text-white dark:hover:bg-indigo-600"
        disabled={!mergeTarget || mergeSaving}
        onclick={commitMerge}
        loading={mergeSaving}
      >
        {m.merge().toLowerCase()} →
      </ButtonEdit>
    </div>
  </div>
</Modal>
