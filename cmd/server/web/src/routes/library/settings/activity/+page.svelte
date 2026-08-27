<script lang="ts">
  import { activityApi, type ActivityEvent } from '$lib/api'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import ActivityList from '$lib/components/ActivityList.svelte'
  import ButtonDownload from '$lib/components/ui/ButtonDownload.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import { m } from '$lib/paraglide/messages'

  let events = $state<ActivityEvent[]>([])
  let loading = $state(true)
  let error = $state('')
  let nextCursor = $state<string | null>(null)
  let hasMore = $state(false)
  let loadingMore = $state(false)

  // Filter state
  let typeFilter = $state<'' | 'uploads' | 'changes' | 'shares'>('')
  let typeFFilters: [string, string][] = [
    ['', '全部'],
    ['uploads', m.uploads()],
    ['changes', m.changes()],
    ['shares', m.shares()],
  ]

  const typesParam: Record<string, string> = {
    uploads: 'asset_created',
    changes:
      'asset_renamed,asset_moved,asset_tagged,asset_untagged,asset_field_set,asset_field_cleared',
    shares: 'asset_shared,asset_share_revoked',
  }

  $effect(() => {
    // Re-fetch when filter changes.
    void typeFilter
    events = []
    nextCursor = null
    hasMore = false
    load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      const res = await activityApi.listWorkspaceActivity({
        limit: 20,
        types: typeFilter ? typesParam[typeFilter] : undefined,
      })
      events = [...res.events]
      nextCursor = res.next_cursor ?? null
      hasMore = res.has_more
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : m.activity_load_failed()
    } finally {
      loading = false
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return
    loadingMore = true
    try {
      const res = await activityApi.listWorkspaceActivity({
        limit: 20,
        cursor: nextCursor,
        types: typeFilter ? typesParam[typeFilter] : undefined,
      })
      events = [...events, ...res.events]
      nextCursor = res.next_cursor ?? null
      hasMore = res.has_more
    } catch {
      /* silently ignore */
    } finally {
      loadingMore = false
    }
  }

  const csvUrl = $derived(activityApi.exportCSV())
</script>

<svelte:head>
  <title>{m.activity()}｜建筑素材中台</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.workspace_activity_title()}
    description={m.workspace_activity_description()}
  >
    <ButtonDownload resourceUrl={csvUrl} text={m.export_csv()} />
  </PageHeader>

  <!-- Filter bar -->
  <div
    class="flex flex-shrink-0 gap-2 border-b border-zinc-100 px-6 py-2 dark:border-zinc-800"
  >
    {#each typeFFilters as [val, label]}
      <Chip
        {label}
        onclick={() => {
          typeFilter = val as typeof typeFilter
        }}
        color={typeFilter === val ? '#6366f1' : undefined}
      />
    {/each}
  </div>

  <!-- Content -->
  <ActivityList
    class="overflow-y-auto"
    {loading}
    {error}
    {events}
    {hasMore}
    {loadMore}
    {loadingMore}
  />
</PageContainer>
