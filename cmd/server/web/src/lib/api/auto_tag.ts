import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type AutoTagSuggestion = definitions['api.AutoTagSuggestionResponse']

export const autoTagApi = {
  /** POST /api/v1/assets/:id/auto-tag (editor+) — manually trigger AI tag suggestions. */
  trigger: (assetId: string) =>
    apiFetch<{ job: string }>(`/api/v1/assets/${assetId}/auto-tag`, {
      method: 'POST',
    }),

  /** GET /api/v1/assets/:id/auto-tag/suggestions — list pending AI tag suggestions. */
  listSuggestions: (assetId: string) =>
    apiFetch<{ suggestions: AutoTagSuggestion[] }>(
      `/api/v1/assets/${assetId}/auto-tag/suggestions`
    ),

  /** POST /api/v1/assets/:id/auto-tag/suggestions/:sid/accept (editor+) — accept one suggestion. */
  acceptSuggestion: (assetId: string, suggestionId: string) =>
    apiFetch<{ name: string }>(
      `/api/v1/assets/${assetId}/auto-tag/suggestions/${suggestionId}/accept`,
      { method: 'POST' }
    ),

  /** POST /api/v1/assets/:id/auto-tag/suggestions/accept-all (editor+) — accept every suggestion. */
  acceptAll: (assetId: string) =>
    apiFetch<{ accepted: number }>(
      `/api/v1/assets/${assetId}/auto-tag/suggestions/accept-all`,
      { method: 'POST' }
    ),

  /** DELETE /api/v1/assets/:id/auto-tag/suggestions/:sid (editor+) — dismiss one suggestion. */
  dismissSuggestion: (assetId: string, suggestionId: string) =>
    apiFetch<void>(
      `/api/v1/assets/${assetId}/auto-tag/suggestions/${suggestionId}`,
      { method: 'DELETE' }
    ),

  /** DELETE /api/v1/assets/:id/auto-tag/suggestions (editor+) — dismiss every suggestion. */
  dismissAll: (assetId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/auto-tag/suggestions`, {
      method: 'DELETE',
    }),
}
