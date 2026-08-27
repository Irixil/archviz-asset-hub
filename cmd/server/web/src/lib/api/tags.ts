import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Tag = definitions['api.TagResponse']
export type DuplicateTagPair = definitions['api.DuplicateTagPairResponse']
export type MergeTagsResult = definitions['api.MergeTagsResponse']
export type BulkDeleteTagsResult = definitions['api.BulkDeleteTagsResponse']

export const tagApi = {
  /** GET /api/v1/tags — list all tags in the workspace (with usage counts). */
  list: (opts?: { system?: boolean; hide_empty?: boolean }) => {
    const params = new URLSearchParams()
    if (opts?.system) params.set('system', 'true')
    if (opts?.hide_empty) params.set('hide_empty', 'true')
    const query = params.toString()
    return apiFetch<Tag[]>(`/api/v1/tags${query ? `?${query}` : ''}`)
  },

  /** POST /api/v1/tags (editor+) — create a tag directly (asset_count = 0). */
  create: (name: string, color?: string | null, group_name?: string | null) =>
    apiFetch<Tag>('/api/v1/tags', {
      method: 'POST',
      body: JSON.stringify({ name, color, group_name }),
    }),

  /** PATCH /api/v1/tags/:name (editor+) — rename, recolor, or regroup a tag. */
  patch: (
    name: string,
    fields: { name?: string; color?: string | null; group_name?: string | null }
  ) =>
    apiFetch<Tag>(`/api/v1/tags/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(fields),
    }),

  /** DELETE /api/v1/tags (editor+) — bulk delete tags by name. */
  bulkDelete: (names: string[]) =>
    apiFetch<BulkDeleteTagsResult>('/api/v1/tags', {
      method: 'DELETE',
      body: JSON.stringify({ names }),
    }),

  /** POST /api/v1/tags/merge (editor+) — merge source tags into target. */
  merge: (sources: string[], target: string) =>
    apiFetch<MergeTagsResult>('/api/v1/tags/merge', {
      method: 'POST',
      body: JSON.stringify({ sources, target }),
    }),

  /** GET /api/v1/tags/suggestions/duplicates — near-duplicate tag pairs. */
  duplicateSuggestions: () =>
    apiFetch<DuplicateTagPair[]>('/api/v1/tags/suggestions/duplicates'),

  /** GET /api/v1/assets/:id/tags — fetch tags assigned to a specific asset. */
  getForAsset: (assetId: string) =>
    apiFetch<string[]>(`/api/v1/assets/${assetId}/tags`),

  /** POST /api/v1/assets/:id/tags (editor+) — add a tag to an asset (auto-creates if needed). */
  addToAsset: (assetId: string, name: string) =>
    apiFetch<{ name: string }>(`/api/v1/assets/${assetId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  /** DELETE /api/v1/assets/:id/tags/:name (editor+) — remove a tag from an asset. */
  removeFromAsset: (assetId: string, name: string) =>
    apiFetch<void>(
      `/api/v1/assets/${assetId}/tags/${encodeURIComponent(name)}`,
      {
        method: 'DELETE',
      }
    ),

  /** POST /api/v1/assets/bulk/tag (editor+) — apply or remove a tag on multiple assets. */
  bulkTag: (
    assetIds: string[],
    tagName: string,
    mode: 'add' | 'remove' = 'add'
  ) =>
    apiFetch<void>('/api/v1/assets/bulk/tag', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, tag_name: tagName, mode }),
    }),
}
