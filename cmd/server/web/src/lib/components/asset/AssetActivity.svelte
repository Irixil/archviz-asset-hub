<script lang="ts">
  import { activityApi, type Asset, type AuditEvent } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import ActivityList from '$lib/components/ActivityList.svelte'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let events = $state<AuditEvent[]>([])
  let loading = $state(false)
  let error = $state('')
  let nextCursor = $state<string | null>(null)
  let hasMore = $state(false)
  let loadingMore = $state(false)

  $effect(() => {
    if (asset?.id) {
      events = []
      nextCursor = null
      hasMore = false
      load()
    }
  })

  async function load() {
    loading = true
    error = ''
    try {
      const res = await activityApi.listAssetEvents(asset.id, { limit: 50 })
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
      const res = await activityApi.listAssetEvents(asset.id, {
        limit: 50,
        cursor: nextCursor,
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
</script>

<ActivityList {loading} {error} {events} {hasMore} {loadMore} {loadingMore} />
