import type { FieldFilter } from '$lib/api'
import {
  assetApi,
  mimeCategory,
  openThumbnailEvents,
  type Asset,
  type AssetListResponse,
} from '$lib/api'
import { canGenerateThumbnail, planAssetUploads } from '$lib/utils/assetFiles'
import { setManualAssetThumbnail } from '$lib/utils/assetThumbnails'
import { navigationStore } from './navigation.svelte'
import { toastStore } from './toast.svelte'
import { uploadsStore } from './uploads.svelte'

let assets = $state<readonly Asset[]>([])
let nextCursor = $state<string | null | undefined>(null)
let loading = $state(false)
let initialLoad = $state(true)
let stale = $state(false)
let query = $state('')
let activeTags = $state<string[]>([])
let similarTo = $state<string | null>(null)
let similarity = $state<AssetListResponse['similarity'] | null>(null)
let similarToNotIndexed = $state(false)
let similarToNoMatches = $state(false)

let sortKey = $state(localStorage.getItem('library-sort-key') ?? 'created_at')
let sortAsc = $state(localStorage.getItem('library-sort-asc') === 'true')
let fieldFilters = $state<FieldFilter[]>([])
let generation = 0
let resetDone = $state(0)

const assetsByCategory = $derived({
  image: assets.filter((a) => mimeCategory(a.mime_type) === 'image'),
  video: assets.filter((a) => mimeCategory(a.mime_type) === 'video'),
  audio: assets.filter((a) => mimeCategory(a.mime_type) === 'audio'),
  document: assets.filter((a) => mimeCategory(a.mime_type) === 'document'),
})

let searchTimer: ReturnType<typeof setTimeout>

// ---- SSE state ----
let sseSource: EventSource | null = null
let sseReconnectTimer: ReturnType<typeof setTimeout> | null = null
let sseReconnectDelay = 1000
let sseReconnectAttempts = 0
const SSE_MAX_ATTEMPTS = 10
// Maps assetId -> uploadId for in-flight thumbnail waits
const pendingThumbnails = new Map<string, string>()

/** Envelope shape for every SSE frame — see internal/events/events.go. */
export interface SSEEvent {
  id: string
  type: string
  payload: {
    asset_id?: string
    variant_id?: string
    workflow_id?: string
    run_id?: string
    node_id?: string
    status?: string
    thumbnail_key?: string
    job_id?: string
    error?: string
    nonce?: string
    preview_url?: string
    expires_at?: string
    reason?: string
  }
}

export const sseEvents = $state<{ last: SSEEvent | null }>({ last: null })

function patchAsset(
  assetId: string,
  patch: Partial<Record<keyof Asset, unknown>>
) {
  assets = assets.map((a) =>
    a.id === assetId ? ({ ...a, ...patch } as Asset) : a
  )
}

function reloadAssetResources(assetId: string) {
  const ts = Date.now()
  const els = document.querySelectorAll<HTMLElement>(
    `[data-asset-dynamic-resource="${assetId}"]`
  )
  const mediaToReload = new Set<HTMLMediaElement>()

  for (const el of els) {
    const tag = el.tagName.toLowerCase()
    if (tag === 'img' || tag === 'iframe') {
      const current = (el as HTMLImageElement | HTMLIFrameElement).src
      const url = new URL(current, window.location.href)
      url.searchParams.set('t', String(ts))
      ;(el as HTMLImageElement | HTMLIFrameElement).src = url.toString()
    } else if (tag === 'source') {
      const current = (el as HTMLSourceElement).src
      const url = new URL(current, window.location.href)
      url.searchParams.set('t', String(ts))
      ;(el as HTMLSourceElement).src = url.toString()
      const parent = el.parentElement
      if (parent instanceof HTMLMediaElement) {
        mediaToReload.add(parent)
      }
    }
  }

  for (const media of mediaToReload) {
    media.load()
  }
}

function connectSSE() {
  if (sseSource && sseSource.readyState !== EventSource.CLOSED) return
  sseSource = openThumbnailEvents()

  sseSource.addEventListener('message', (e: MessageEvent) => {
    console.debug('[sse] message received', e.data)
    try {
      const event = JSON.parse(e.data) as SSEEvent
      sseEvents.last = event
      sseReconnectDelay = 1000
      sseReconnectAttempts = 0

      if (event.type === 'resync') {
        // Last-Event-ID fell out of the server's replay buffer — refetch
        // instead of trusting the stream to fill the gap.
        load(true)
        return
      }

      if (event.type !== 'thumbnail_ready') return
      const assetId = event.payload.asset_id
      if (!assetId) return

      reloadAssetResources(assetId)
      patchAsset(assetId, { thumbnail_key: event.payload.thumbnail_key })

      const uploadId = pendingThumbnails.get(assetId)
      if (uploadId) {
        pendingThumbnails.delete(assetId)
        uploadsStore.update(uploadId, { status: 'done' })
      }
    } catch {
      console.warn(`unhandled event`, e)
      // ignore malformed events
    }
  })

  sseSource.onerror = () => {
    sseSource?.close()
    sseSource = null
    if (sseReconnectAttempts >= SSE_MAX_ATTEMPTS) return
    sseReconnectAttempts++
    if (sseReconnectTimer) clearTimeout(sseReconnectTimer)
    sseReconnectTimer = setTimeout(() => {
      sseReconnectTimer = null
      sseReconnectDelay = Math.min(sseReconnectDelay * 2, 30000)
      connectSSE()
    }, sseReconnectDelay)
  }
}

async function pollThumbnail(assetId: string, uploadId: string, retries = 5) {
  for (let i = 0; i < retries; i++) {
    await new Promise((r) => setTimeout(r, 3000))
    try {
      const asset = await assetApi.get(assetId)
      if (asset.thumbnail_key) {
        patchAsset(assetId, { thumbnail_key: asset.thumbnail_key })
        uploadsStore.update(uploadId, { status: 'done' })
        return
      }
    } catch {
      // continue retrying
    }
  }
  // Exhausted retries — mark done so the spinner doesn't stay forever
  uploadsStore.update(uploadId, { status: 'done' })
}

connectSSE()

async function load(reset = false) {
  if (!reset && loading) return
  if (reset) {
    generation++
    nextCursor = null
    stale = false
  }
  const myGen = generation
  loading = true
  try {
    const result = await assetApi.list({
      cursor: nextCursor ?? undefined,
      sortKey: sortKey || undefined,
      sortAsc: !!sortAsc,
      q: query || undefined,
      project_id: navigationStore.activeProjectId ?? undefined,
      tags: activeTags.length > 0 ? activeTags : undefined,
      folder_id: navigationStore.activeFolderId ?? undefined,
      collection_id: navigationStore.activeCollectionId ?? undefined,
      similar_to: similarTo ?? undefined,
      fieldFilters: fieldFilters.length > 0 ? fieldFilters : undefined,
      limit: 20,
    })
    if (myGen !== generation) return
    assets = reset ? result.assets : [...assets, ...result.assets]
    nextCursor = result.next_cursor
    similarity = result.similarity ?? null
    similarToNotIndexed = !!result.similar_to_not_indexed
    similarToNoMatches = !!result.similar_to_no_matches
    if (reset) resetDone++
  } catch {
    // 401 redirect handled by api client
  } finally {
    if (myGen === generation) loading = false
    initialLoad = false
  }
}

export const assetsStore = {
  get assets() {
    return assets
  },
  get nextCursor() {
    return nextCursor
  },
  get loading() {
    return loading
  },
  get initialLoad() {
    return initialLoad
  },
  get stale() {
    return stale
  },
  get query() {
    return query
  },
  set query(v: string) {
    query = v
  },
  get activeTags() {
    return activeTags
  },
  get similarTo() {
    return similarTo
  },
  get similarity() {
    return similarity
  },
  get similarToNotIndexed() {
    return similarToNotIndexed
  },
  get similarToNoMatches() {
    return similarToNoMatches
  },
  get fieldFilters() {
    return fieldFilters
  },
  get assetsByCategory() {
    return assetsByCategory
  },
  get resetDone() {
    return resetDone
  },

  invalidate() {
    load(true)
  },

  load,

  get sortKey() {
    return sortKey
  },
  get sortAsc() {
    return sortAsc
  },

  sort(key: string, asc: boolean) {
    sortKey = key
    sortAsc = asc
    localStorage.setItem('library-sort-key', key)
    localStorage.setItem('library-sort-asc', String(asc))
    load(true)
  },

  search(query: string) {
    assetsStore.query = query
    clearTimeout(searchTimer)
    nextCursor = null
    searchTimer = setTimeout(() => {
      load(true)
    }, 300)
  },

  setActiveTags(tags: string[], options: { load?: boolean } = {}) {
    activeTags = tags
    nextCursor = null
    if (options.load !== false) load(true)
  },

  setSimilarTo(assetId: string | null, options: { load?: boolean } = {}) {
    similarTo = assetId
    similarity = null
    similarToNotIndexed = false
    similarToNoMatches = false
    nextCursor = null
    if (options.load !== false) load(true)
  },

  clearSimilarity(options: { load?: boolean } = {}) {
    assetsStore.setSimilarTo(null, options)
  },

  setFieldFilters(filters: FieldFilter[]) {
    fieldFilters = filters
    nextCursor = null
    load(true)
  },

  prepend(asset: Asset) {
    assets = [asset, ...assets]
  },

  remove(id: string) {
    assets = assets.filter((a) => a.id !== id)
  },

  upload(files: File[], projectId: string | null, folderId: string | null) {
    for (const { file, thumbnail } of planAssetUploads(files)) {
      const id = crypto.randomUUID()
      uploadsStore.add({ id, file, progress: 0, status: 'uploading' })
      assetApi
        .upload(file, projectId, folderId, (pct) =>
          uploadsStore.update(id, { progress: pct })
        )
        .then(async (asset) => {
          assetsStore.prepend(asset)
          uploadsStore.update(id, { status: 'processing', asset })

          if (thumbnail) {
            try {
              const updated = await setManualAssetThumbnail(asset.id, thumbnail)
              patchAsset(asset.id, updated)
              reloadAssetResources(asset.id)
              uploadsStore.update(id, { status: 'done', asset: updated })
              toastStore.show(
                `已将 ${thumbnail.name} 设为 ${file.name} 的缩略图`,
                'success'
              )
            } catch (error) {
              uploadsStore.update(id, { status: 'done', asset })
              toastStore.show(
                `${file.name} 已上传，但自动添加缩略图失败：${
                  error instanceof Error ? error.message : '请稍后重试'
                }`,
                'error'
              )
            }
            return
          }

          if (
            asset.thumbnail_key ||
            !canGenerateThumbnail(asset.original_filename, asset.mime_type)
          ) {
            uploadsStore.update(id, { status: 'done', asset })
            return
          }

          // Register for SSE notification
          pendingThumbnails.set(asset.id, id)

          // 30s timeout: fall back to polling if SSE hasn't delivered
          setTimeout(() => {
            if (pendingThumbnails.has(asset.id)) {
              pendingThumbnails.delete(asset.id)
              pollThumbnail(asset.id, id)
            }
          }, 30_000)
        })
        .catch((err: Error) =>
          uploadsStore.update(id, { status: 'error', error: err.message })
        )
    }
  },

  patchAsset,

  reloadAssetResources,

  bulkProject: async (assetIds: string[], projectId: string | null) => {
    await assetApi.bulkProject(assetIds, projectId)
  },

  bulkDelete: async (assetIds: string[]) => {
    await assetApi.bulkDelete(assetIds)
  },

  addTag(assetId: string, tag: string) {
    assets = assets.map((a) =>
      a.id === assetId ? { ...a, tags: [...(a.tags ?? []), tag] } : a
    )
  },

  removeTag(assetId: string, tag: string) {
    assets = assets.map((a) =>
      a.id === assetId
        ? { ...a, tags: (a.tags ?? []).filter((t) => t !== tag) }
        : a
    )
  },

  /**
   * No-op: Asset list rows don't carry field values (fetched on-demand in lightbox).
   * Signature kept so BulkMetadataCommand compiles and future caching is easy to add.
   */
  patchFieldValues(
    _assetId: string,
    _edits: { fieldId: string; value: string | number | boolean | null }[]
  ) {},
}
