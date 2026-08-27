import type { Asset } from './assets'
import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Collection = definitions['api.CollectionResponse']

export const collectionApi = {
  /** GET /api/v1/collections — list collections in the workspace. */
  list: (): Promise<Collection[]> => apiFetch('/api/v1/collections'),

  /** POST /api/v1/collections (editor+) — create a new collection. */
  create: (
    name: string,
    description = '',
    assetIds: string[] = []
  ): Promise<Collection> =>
    apiFetch('/api/v1/collections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description, asset_ids: assetIds }),
    }),

  /** GET /api/v1/collections/:id — fetch a collection with its assets. */
  get: (id: string): Promise<Collection & { assets: Asset[] }> =>
    apiFetch(`/api/v1/collections/${id}`),

  /** PUT /api/v1/collections/:id (editor+) — update collection name or description. */
  update: (id: string, name: string, description = ''): Promise<Collection> =>
    apiFetch(`/api/v1/collections/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }),

  /** DELETE /api/v1/collections/:id (owner only) — delete a collection. */
  delete: (id: string): Promise<void> =>
    apiFetch(`/api/v1/collections/${id}`, { method: 'DELETE' }),

  /** POST /api/v1/collections/:id/assets/:aid (editor+) — add an asset to a collection. */
  addAsset: (collectionId: string, assetId: string): Promise<void> =>
    apiFetch(`/api/v1/collections/${collectionId}/assets/${assetId}`, {
      method: 'POST',
    }),

  /** DELETE /api/v1/collections/:id/assets/:aid (editor+) — remove an asset from a collection. */
  removeAsset: (collectionId: string, assetId: string): Promise<void> =>
    apiFetch(`/api/v1/collections/${collectionId}/assets/${assetId}`, {
      method: 'DELETE',
    }),

  /** GET /api/v1/assets/:id/collections — list collections an asset belongs to. */
  forAsset: (assetId: string): Promise<Collection[]> =>
    apiFetch(`/api/v1/assets/${assetId}/collections`),
}
