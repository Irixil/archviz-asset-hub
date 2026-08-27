<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { onMount } from 'svelte'
  import StorageBar from '$lib/components/storage/StorageBar.svelte'
  import {
    fetchWorkspaceStorage,
    type WorkspaceStorageUsage,
  } from '$lib/api/storage'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import { BulkAssignAssetToFolder } from '$lib/commands/BulkAssignAssetToFolder'
  import { BulkAssignAssetToProject } from '$lib/commands/BulkAssignAssetToProject'
  import { m } from '$lib/paraglide/messages'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import {
    ARCHITECTURE_TAXONOMY,
    normalizeTaxonomyTagName,
    primaryFacetForGroup,
    type ArchiveGroupId,
  } from '$lib/data/architectureTaxonomy'
  import {
    Activity,
    ArrowLeft,
    Check,
    ChevronDown,
    Download,
    HardDrive,
    PackageOpen,
    PanelsTopLeft,
    Plus,
    ScanSearch,
    Settings2,
    Upload,
    Tags,
    User,
    Users,
  } from '@lucide/svelte'

  interface Props {
    onNavigate?: () => void
  }

  let { onNavigate = () => {} }: Props = $props()

  let sidebarCreating = $state(false)
  let storageUsage = $state<WorkspaceStorageUsage | null>(null)

  const assetArchive = ARCHITECTURE_TAXONOMY.find(
    (group) => group.id === 'assets'
  )!
  const assetArchiveActive = $derived(
    navigationStore.activeArchiveGroupId === 'assets'
  )

  async function loadStorageUsage() {
    storageUsage = await fetchWorkspaceStorage().catch(() => null)
  }

  onMount(() => {
    loadStorageUsage()
    const interval = setInterval(loadStorageUsage, 60_000)
    return () => clearInterval(interval)
  })

  const profileSections = [
    {
      id: 'account',
      label: () => m.settings_account_title(),
      path: '/library/settings/account',
      icon: User,
    },
  ]

  const settingsSections = [
    {
      id: 'members',
      label: () => m.tab_members(),
      path: '/library/settings/members',
      icon: Users,
    },
    {
      id: 'tags',
      label: () => m.tags(),
      path: '/library/settings/tags',
      icon: Tags,
    },
    {
      id: 'custom-fields',
      label: () => m.custom_fields_title(),
      path: '/library/settings/custom-fields',
      icon: Settings2,
    },
    {
      id: 'ingress',
      label: () => m.tab_ingress(),
      path: '/library/settings/ingress',
      icon: Download,
    },
    {
      id: 'exports',
      label: () => '素材导出',
      path: '/library/settings/exports',
      icon: Upload,
    },
    {
      id: 'content',
      label: () => m.settings_content_title(),
      path: '/library/settings/content',
      icon: ScanSearch,
    },
    {
      id: 'storage',
      label: () => m.storage_breakdown_title(),
      path: '/library/settings/storage',
      icon: HardDrive,
    },
  ]

  const securitySections = [
    {
      id: 'activity',
      label: () => m.activity(),
      path: '/library/settings/activity',
      icon: Activity,
    },
  ]

  const isSettings = $derived(page.url.pathname.startsWith('/library/settings'))
  const activeSettingsSection = $derived(
    [...profileSections, ...settingsSections, ...securitySections].find((s) =>
      page.url.pathname.startsWith(s.path)
    )?.id ?? null
  )

  async function handleProjectSelect(id: string | null) {
    const changed =
      navigationStore.activeProjectId !== id ||
      navigationStore.activeArchiveGroupId !== null
    assetsStore.setActiveTags([], { load: false })
    navigationStore.selectProject(id)
    if (id) await foldersStore.loadForProject(id)
    if (!changed) assetsStore.load(true)
    await goto('/library')
    onNavigate()
  }

  async function handleFolderSelect(
    _projectId: string,
    folderId: string | null
  ) {
    navigationStore.selectFolder(folderId)
    await goto('/library')
    onNavigate()
  }

  async function handleAssetsDropped(
    assetIds: string[],
    folderId: string | null,
    projectId: string
  ) {
    try {
      const folder =
        foldersStore.foldersForActiveProject.find((f) => f.id === folderId) ??
        null
      await undoStore.execute(
        new BulkAssignAssetToFolder(
          assetsStore.assets.filter((asset) => assetIds.includes(asset.id)),
          folderId ?? null,
          folder?.name ?? null,
          projectId
        )
      )
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  async function handleAssetsProjectDropped(
    assetIds: string[],
    projectId: string
  ) {
    const beforeProjectIds = new Map(
      assetIds.map((id) => [
        id,
        assetsStore.assets.find((asset) => asset.id === id)?.project_id ?? null,
      ])
    )
    const projectName =
      projectsStore.projects.find((project) => project.id === projectId)
        ?.name ?? null

    try {
      await undoStore.execute(
        new BulkAssignAssetToProject(
          assetIds,
          beforeProjectIds,
          projectId,
          projectName
        )
      )
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  async function handleArchiveGroupSelect(id: ArchiveGroupId) {
    const changed = navigationStore.activeArchiveGroupId !== id
    assetsStore.setActiveTags([], { load: false })
    navigationStore.selectArchiveGroup(id)
    if (!changed) assetsStore.load(true)
    await goto('/library')
    onNavigate()
  }

  async function handleArchiveSubsetSelect(
    groupId: ArchiveGroupId,
    entry: string
  ) {
    const selected = assetsStore.activeTags.some(
      (tag) => normalizeTaxonomyTagName(tag) === normalizeTaxonomyTagName(entry)
    )
    navigationStore.selectArchiveGroup(groupId)
    assetsStore.setActiveTags(selected ? [] : [entry])
    await goto('/library')
    onNavigate()
  }

  function handleAnchorNavigate() {
    onNavigate()
  }
</script>

<aside
  class="library-sidebar relative flex h-full w-full shrink-0 flex-col overflow-y-auto border-r border-[var(--border-subtle)] bg-[var(--bg-sidebar)] md:w-64 xl:w-72"
>
  <div class="flex h-16 shrink-0 items-center gap-3 px-4">
    <div
      class="flex h-9 w-9 items-center justify-center rounded-xl bg-[var(--accent-cta)] text-[var(--selection-bar-text)] shadow-sm"
    >
      <PanelsTopLeft class="h-4.5 w-4.5" />
    </div>
    <div class="min-w-0">
      <p class="truncate text-sm font-semibold text-[var(--text-primary)]">
        建筑素材中台
      </p>
      <p class="truncate text-xs text-[var(--text-muted)]">内部制作资源</p>
    </div>
  </div>

  {#if isSettings}
    <WorkspaceSwitcher class="px-3 py-3" />

    <div class="px-3 pb-2">
      <a
        href="/library"
        class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={handleAnchorNavigate}
      >
        <ArrowLeft class="h-4 w-4 shrink-0" />
        <span>{m.back_to_library()}</span>
      </a>
    </div>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.user_profile()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each profileSections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.workspace()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each settingsSections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.privacy_audit_logs()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each securitySections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>
  {:else}
    <WorkspaceSwitcher class="px-3 py-3" />

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-3">
      <div class="mb-2 px-2">
        <span
          class="text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
          >资料库</span
        >
      </div>

      <nav class="flex flex-col gap-0.5" aria-label="资料库分类">
        <button
          type="button"
          aria-current={assetArchiveActive ? 'page' : undefined}
          aria-expanded={assetArchiveActive}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {assetArchiveActive
            ? 'bg-[var(--selection-bar-bg)] font-semibold text-[var(--selection-bar-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={() => handleArchiveGroupSelect('assets')}
        >
          <PackageOpen
            class="h-4 w-4 shrink-0 {assetArchiveActive
              ? 'text-[var(--selection-bar-text)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{assetArchive.label}</span>
          {#if authStore.totalAssetCount > 0}
            <span
              class="shrink-0 text-xs tabular-nums {assetArchiveActive
                ? 'text-[var(--selection-bar-text)]/70'
                : 'text-[var(--text-muted)]'}">{authStore.totalAssetCount}</span
            >
          {/if}
          <ChevronDown
            class="h-3.5 w-3.5 shrink-0 transition-transform {assetArchiveActive
              ? 'rotate-0'
              : '-rotate-90'}"
          />
        </button>

        {#if assetArchiveActive}
          {@const primaryFacet = primaryFacetForGroup(assetArchive)}
          <div
            class="mt-1 mb-1 ml-5 flex flex-col gap-0.5 border-l border-[var(--border)] pl-2"
            aria-label={`${assetArchive.label}${primaryFacet.label}`}
          >
            {#each primaryFacet.entries as entry (entry)}
              {@const childSelected = assetsStore.activeTags.some(
                (tag) =>
                  normalizeTaxonomyTagName(tag) ===
                  normalizeTaxonomyTagName(entry)
              )}
              <button
                type="button"
                aria-pressed={childSelected}
                class="flex min-h-8 w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-xs transition-colors
                {childSelected
                  ? 'bg-[var(--accent-cta)] font-semibold text-white'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
                onclick={() => handleArchiveSubsetSelect('assets', entry)}
              >
                {#if childSelected}
                  <Check class="h-3.5 w-3.5 shrink-0" />
                {:else}
                  <span
                    class="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--text-muted)]"
                  ></span>
                {/if}
                <span class="min-w-0 flex-1 truncate text-left">{entry}</span>
              </button>
            {/each}
          </div>
        {/if}
      </nav>
    </div>

    <div
      class="mb-4 flex flex-col overflow-hidden border-t border-[var(--border-subtle)] px-3 pt-3"
    >
      <div class="mb-2 flex items-center justify-between px-2">
        <span
          class="text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
          >{m.projects()}</span
        >
        {#if authStore.role !== 'viewer'}
          <button
            class="rounded p-0.5 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
            onclick={() => {
              sidebarCreating = true
            }}
            aria-label={m.new_project()}
          >
            <Plus class="h-3.5 w-3.5" />
          </button>
        {/if}
      </div>

      <nav class="flex-1 overflow-y-auto">
        <ProjectSidebar
          selectedAssetIds={selectionStore.selectedIds}
          creating={sidebarCreating}
          onCreatingChange={(value) => {
            sidebarCreating = value
          }}
          onSelect={handleProjectSelect}
          onFolderSelect={handleFolderSelect}
          onAssetsFolderDropped={handleAssetsDropped}
          onAssetsProjectDropped={handleAssetsProjectDropped}
        />
      </nav>
    </div>

    {#if storageUsage}
      <a class="mt-auto" href="/library/settings/storage">
        <div class="border-t border-[var(--border-subtle)] px-4 py-2">
          <StorageBar
            used={storageUsage.total_bytes}
            limit={storageUsage.limit_bytes}
            compact
          />
        </div>
      </a>
    {/if}
  {/if}
</aside>

<style>
  .library-sidebar {
    --bg-sidebar: #1d2421;
    --bg-surface: #252d29;
    --bg-elevated: #2b342f;
    --bg-stripe: #222925;
    --bg-hover: #303a35;
    --border: #455049;
    --border-default: #455049;
    --border-subtle: #343e38;
    --text-primary: #f1f2ed;
    --text-secondary: #c2cbc5;
    --text-muted: #93a198;
    --accent: #d8a14b;
    --accent-hover: #e4b365;
    --accent-soft: rgb(216 161 75 / 0.14);
    --accent-text: #efc981;
    --accent-cta: #d8a14b;
    --accent-cta-hover: #e4b365;
    --selection-bar-bg: #d8a14b;
    --selection-bar-text: #211a10;
    color-scheme: dark;
  }

  .library-sidebar :global(button:focus-visible),
  .library-sidebar :global(a:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
