import type { AIProvider } from './ai_providers'
import { ApiError, apiFetch } from './client'
import type { FieldFilter } from './models'
import type { definitions } from './types.gen'

export type AssetListResponse = definitions['api.AssetListResponse']
export type ShareComment = definitions['api.CommentResponse']
export type VisualSimilarResult = definitions['api.VisualSimilarResult']
export type SharedVariant = definitions['api.SharedVariantResponse']

export type Asset = Omit<
  definitions['api.AssetResponse'],
  'classification_status' | 'classification_category'
> & {
  storage_key?: string
  classification_status?: string
  classification_category?: string
  created_by?: { id: string; name: string } | null
  authors?: { id: string; name: string }[]
}
export type PublicAsset = Asset
export type DuplicateOf = definitions['api.DuplicateOfResponse']

const API_BASE = import.meta.env.VITE_API_URL ?? ''

let aiProvidersCache: AIProvider[] | null = null
let aiProvidersPromise: Promise<AIProvider[]> | null = null

export function invalidateAIProviderModelsCache() {
  aiProvidersCache = null
  aiProvidersPromise = null
}

async function fetchProvidersCached(): Promise<AIProvider[]> {
  if (aiProvidersCache) return aiProvidersCache
  if (aiProvidersPromise) return aiProvidersPromise

  aiProvidersPromise = apiFetch<{ providers: AIProvider[] }>(
    '/api/v1/aiproviders'
  ).then((r) => r.providers)

  try {
    const response = await aiProvidersPromise
    aiProvidersCache = response
    return response
  } catch (error) {
    aiProvidersPromise = null
    throw error
  } finally {
    if (aiProvidersCache) {
      aiProvidersPromise = null
    }
  }
}

export const assetApi = {
  /** POST /api/v1/assets (editor+) — upload a new asset via XHR (with progress callback). */
  upload(
    file: File,
    projectId: string | null,
    folderId: string | null,
    onProgress?: (pct: number) => void
  ): Promise<Asset> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${API_BASE}/api/v1/assets`)
      xhr.withCredentials = true

      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) {
            onProgress(Math.round((e.loaded / e.total) * 100))
          }
        })
      }

      xhr.addEventListener('load', () => {
        if (xhr.status === 201) {
          resolve(JSON.parse(xhr.responseText) as Asset)
        } else if (xhr.status === 401 && typeof window !== 'undefined') {
          window.location.href = '/login'
          reject(new ApiError(401, 'Unauthorized'))
        } else {
          const body = JSON.parse(xhr.responseText) as {
            error?: string
            duplicate_of?: DuplicateOf
          }
          reject(new ApiError(xhr.status, body.error ?? xhr.statusText, body))
        }
      })

      xhr.addEventListener('error', () =>
        reject(new ApiError(0, 'Network error'))
      )

      const fd = new FormData()
      fd.append('file', file)
      if (projectId) fd.append('project_id', projectId)
      if (folderId) fd.append('folder_id', folderId)
      xhr.send(fd)
    })
  },

  /** GET /api/v1/assets — list assets with pagination, search, filtering, sorting. */
  list(
    params: {
      cursor?: string
      limit?: number
      sortKey?: string
      sortAsc?: boolean
      q?: string
      project_id?: string
      mime?: string
      tags?: string[]
      folder_id?: string
      collection_id?: string
      similar_to?: string
      fieldFilters?: FieldFilter[]
    } = {}
  ): Promise<AssetListResponse> {
    const qs = new URLSearchParams()
    if (params.sortKey)
      qs.set('sort', `${params.sortKey}_${params.sortAsc ? 'asc' : 'desc'}`)
    if (params.cursor) qs.set('cursor', params.cursor)
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.q) qs.set('q', `"${params.q.replaceAll('"', '""')}"`)
    if (params.project_id) qs.set('project_id', params.project_id)
    if (params.mime) qs.set('mime', params.mime)
    if (params.tags && params.tags.length > 0)
      qs.set('tags', params.tags.join(','))
    if (params.folder_id) qs.set('folder_id', params.folder_id)
    if (params.collection_id) qs.set('collection_id', params.collection_id)
    if (params.similar_to) qs.set('similar_to', params.similar_to)
    if (params.fieldFilters) {
      for (const f of params.fieldFilters) {
        const paramKey =
          f.op === 'eq' ? `field[${f.key}]` : `field[${f.key}][${f.op}]`
        qs.set(paramKey, f.value)
      }
    }
    const query = qs.toString()
    return apiFetch<AssetListResponse>(
      `/api/v1/assets${query ? '?' + query : ''}`
    )
  },

  /** GET /api/v1/assets/:id — fetch asset metadata by ID. */
  get(id: string): Promise<Asset> {
    return apiFetch<Asset>(`/api/v1/assets/${id}`)
  },

  /** POST /api/v1/assets/:id/classify — retry format-level classification. */
  classify: (id: string) =>
    apiFetch<{
      classification_status: 'classified'
      classification_category: string
    }>(`/api/v1/assets/${id}/classify`, { method: 'POST' }),

  /** PATCH /api/v1/assets/:id (editor+) — move asset to a folder (pass null to unassign). */
  updateFolder: (assetId: string, folderId: string | null) =>
    apiFetch<Asset>(`/api/v1/assets/${assetId}`, {
      method: 'PATCH',
      body: JSON.stringify({ folder_id: folderId }),
    }),

  /** PUT /api/v1/assets/:id/rename (editor+) — rename an asset. */
  rename: (assetId: string, name: string) =>
    apiFetch<Asset>(`/api/v1/assets/${assetId}/rename`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  /** DELETE /api/v1/assets/:id (editor+) — delete an asset. */
  delete(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/assets/${id}`, { method: 'DELETE' })
  },

  /** POST /api/v1/assets/bulk/project (editor+) — assign multiple assets to a project. */
  bulkProject: (assetIds: string[], projectId: string | null) =>
    apiFetch<void>('/api/v1/assets/bulk/project', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, project_id: projectId }),
    }),

  /** DELETE /api/v1/assets/bulk (owner only) — delete multiple assets. */
  bulkDelete: (assetIds: string[]) =>
    apiFetch<void>('/api/v1/assets/bulk', {
      method: 'DELETE',
      body: JSON.stringify({ asset_ids: assetIds }),
    }),

  /** GET /api/v1/assets/:id/thumb — thumbnail URL for an asset.
   * Pass thumbnailKey to append a cache-busting version token so the browser
   * refetches when the thumbnail is regenerated (24h max-age otherwise wins). */
  thumbUrl(id: string, thumbnailKey?: string | null): string {
    const base = `${API_BASE}/api/v1/assets/${id}/thumb`
    if (!thumbnailKey) return base
    // Use the last path segment of the storage key as an opaque version token.
    const v = thumbnailKey.split('/').pop() ?? thumbnailKey
    return `${base}?v=${encodeURIComponent(v)}`
  },

  /** GET /api/v1/assets/:id/file — original file URL for an asset. */
  fileUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/file`
  },

  /** GET /api/v1/assets/:id/comments — list comments for an asset. */
  listAssetComments: (id: string) =>
    apiFetch<ShareComment[]>(`/api/v1/assets/${id}/comments`),

  /** POST /api/v1/assets/:id/thumb/regenerate (editor+) — requeue the thumbnail generation job. */
  regenerateThumbnail: (id: string) =>
    apiFetch<{ job_id: string; status: string; message: string }>(
      `/api/v1/assets/${id}/thumb/regenerate`,
      {
        method: 'POST',
      }
    ),

  /** GET /api/v1/aiprovides — list image-to-image models via the authenticated backend proxy. */
  fetchProviders: () => fetchProvidersCached(),

  /** GET /api/v1/assets/:id/similar — find visually similar image assets. */
  findSimilar: (id: string) =>
    apiFetch<{ results: VisualSimilarResult[] }>(
      `/api/v1/assets/${id}/similar`
    ),
}
