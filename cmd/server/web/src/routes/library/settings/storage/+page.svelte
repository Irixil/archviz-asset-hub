<script lang="ts">
  import { onMount } from 'svelte'
  import { ChevronDown, ChevronUp } from '@lucide/svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import SectionHeading from '$lib/components/ui/SectionHeading.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import StorageBar from '$lib/components/storage/StorageBar.svelte'
  import { DOT_COLORS } from '$lib/stores/assetView'
  import {
    fetchWorkspaceStorage,
    fetchProjectFolderStorage,
    type WorkspaceStorageUsage,
    type FolderStorageUsage,
  } from '$lib/api/storage'
  import { formatBytes } from '$lib/api/client'
  import { m } from '$lib/paraglide/messages'

  let usage = $state<WorkspaceStorageUsage | null>(null)
  let loading = $state(true)
  let expandedProjects = $state<Record<string, FolderStorageUsage[] | null>>({})
  let loadingProjects = $state<Record<string, boolean>>({})

  async function load() {
    loading = true
    usage = await fetchWorkspaceStorage().catch(() => null)
    loading = false
  }

  async function toggleProject(projectId: string | null) {
    if (projectId == null) return
    if (expandedProjects[projectId] !== undefined) {
      const next = { ...expandedProjects }
      delete next[projectId]
      expandedProjects = next
      return
    }
    loadingProjects = { ...loadingProjects, [projectId]: true }
    try {
      const res = await fetchProjectFolderStorage(projectId)
      expandedProjects = { ...expandedProjects, [projectId]: res.folders }
    } catch {
      expandedProjects = { ...expandedProjects, [projectId]: [] }
    } finally {
      const next = { ...loadingProjects }
      delete next[projectId]
      loadingProjects = next
    }
  }

  function timeAgo(iso: string): string {
    const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
    if (diff < 60) return `${diff} 秒前`
    if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
    return `${Math.floor(diff / 3600)} 小时前`
  }

  const typeTwColors: Record<string, string> = {
    image: DOT_COLORS.image,
    video: DOT_COLORS.video,
    audio: DOT_COLORS.audio,
    document: DOT_COLORS.document,
    other: 'bg-gray-300 dark:bg-gray-600',
  }

  const typeLabels: Record<string, () => string> = {
    image: () => m.storage_breakdown_image(),
    video: () => m.storage_breakdown_video(),
    audio: () => m.storage_breakdown_audio(),
    document: () => m.storage_breakdown_document(),
    other: () => m.storage_breakdown_other(),
  }

  const typeBuckets = ['image', 'video', 'audio', 'document', 'other'] as const

  onMount(load)
</script>

<svelte:head>
  <title>{m.storage_breakdown_title()}｜建筑素材中台</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.storage_breakdown_title()}
    description={usage
      ? m.storage_breakdown_last_updated({ ago: timeAgo(usage.computed_at) })
      : ''}
  />

  <div class="mb-8 flex-1 overflow-y-auto px-6 pt-6 pb-8">
    <div class="mx-auto w-full max-w-4xl space-y-8">
      {#if loading}
        <div class="flex justify-center py-16">
          <Spinner size="md" />
        </div>
      {:else if !usage}
        <EmptyState title={m.storage_load_failed()} />
      {:else}
        <!-- Summary bar -->
        <div
          class="space-y-3 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
        >
          <StorageBar
            height={20}
            used={usage.total_bytes}
            limit={usage.limit_bytes}
          />
          <Hint>
            {formatBytes(usage.versions_bytes)}
            {m.storage_breakdown_versions()} ·
            {formatBytes(usage.variants_bytes)}
            {m.storage_breakdown_variants()}
          </Hint>
        </div>

        <!-- By-project table -->
        <div
          class="overflow-hidden rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] shadow-sm"
        >
          <table class="w-full text-sm">
            <thead>
              <tr
                class="border-b border-[var(--border-subtle)] text-xs tracking-wide text-[var(--text-muted)] uppercase"
              >
                <th class="px-4 py-3 text-left font-medium"
                  >{m.storage_breakdown_project()}</th
                >
                <th class="px-4 py-3 text-right font-medium"
                  >{m.storage_breakdown_versions()}</th
                >
                <th class="px-4 py-3 text-right font-medium"
                  >{m.storage_breakdown_variants()}</th
                >
                <th class="px-4 py-3 text-right font-medium"
                  >{m.storage_breakdown_total()}</th
                >
                <th class="w-8 px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {#each usage.by_project as project}
                <tr
                  class="border-b border-[var(--border-subtle)] last:border-0 hover:bg-[var(--bg-hover)]"
                >
                  <td class="px-4 py-3 font-medium text-[var(--text-primary)]">
                    {project.project_name || m.storage_breakdown_unassigned()}
                  </td>
                  <td class="px-4 py-3 text-right text-[var(--text-secondary)]"
                    >{formatBytes(project.versions_bytes)}</td
                  >
                  <td class="px-4 py-3 text-right text-[var(--text-secondary)]"
                    >{formatBytes(project.variants_bytes)}</td
                  >
                  <td
                    class="px-4 py-3 text-right font-medium text-[var(--text-primary)]"
                    >{formatBytes(project.total_bytes)}</td
                  >
                  <td class="px-4 py-3 text-center">
                    {#if project.project_id && project.folder_count > 0}
                      <button
                        class="text-[var(--text-muted)] transition-colors hover:text-[var(--text-primary)]"
                        onclick={() =>
                          project.folder_count > 0 &&
                          toggleProject(project.project_id)}
                        aria-label="Toggle folder breakdown"
                      >
                        {#if expandedProjects[project.project_id!] !== undefined}
                          <ChevronUp class="h-4 w-4" />
                        {:else}
                          <ChevronDown class="h-4 w-4" />
                        {/if}
                      </button>
                    {/if}
                  </td>
                </tr>

                {#if project.project_id && expandedProjects[project.project_id] !== undefined}
                  {#if loadingProjects[project.project_id]}
                    <tr>
                      <td
                        colspan="5"
                        class="px-8 py-2 text-xs text-[var(--text-muted)]"
                        >Loading…</td
                      >
                    </tr>
                  {:else}
                    {#each expandedProjects[project.project_id!] ?? [] as folder}
                      <tr
                        class="border-b border-[var(--border-subtle)] bg-[var(--bg-elevated)] last:border-0"
                      >
                        <td class="px-8 py-2 text-[var(--text-secondary)]">
                          {folder.folder_name || m.storage_breakdown_root()}
                        </td>
                        <td
                          class="px-4 py-2 text-right text-xs text-[var(--text-muted)]"
                          >{formatBytes(folder.versions_bytes)}</td
                        >
                        <td
                          class="px-4 py-2 text-right text-xs text-[var(--text-muted)]"
                          >{formatBytes(folder.variants_bytes)}</td
                        >
                        <td
                          class="px-4 py-2 text-right text-xs text-[var(--text-secondary)]"
                          >{formatBytes(folder.total_bytes)}</td
                        >
                        <td></td>
                      </tr>
                    {/each}
                  {/if}
                {/if}
              {/each}
            </tbody>
          </table>
        </div>

        <!-- By-type stacked bar -->
        {#if usage.total_bytes > 0}
          <div
            class="space-y-4 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
          >
            <SectionHeading title={m.storage_breakdown_by_type()} />
            <div class="flex h-5 w-full overflow-hidden rounded-xs">
              {#each typeBuckets as bucket}
                {@const bytes = usage.by_type[bucket]}
                {#if bytes > 0}
                  <div
                    class={typeTwColors[bucket]}
                    style="width: {(bytes / usage.total_bytes) * 100}%"
                    title="{typeLabels[bucket]()}: {formatBytes(bytes)}"
                  ></div>
                {/if}
              {/each}
            </div>
            <div class="flex flex-wrap gap-4">
              {#each typeBuckets as bucket}
                {@const bytes = usage.by_type[bucket]}
                <div class="flex items-center gap-2">
                  <span class="h-2.5 w-2.5 rounded-full {typeTwColors[bucket]}"
                  ></span>
                  <span class="text-xs text-[var(--text-secondary)]">
                    {typeLabels[bucket]()}
                    <span class="text-[var(--text-muted)]"
                      >({formatBytes(bytes)})</span
                    >
                  </span>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      {/if}
    </div>
  </div>
</PageContainer>
