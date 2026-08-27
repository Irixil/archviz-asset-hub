import { ApiError, apiFetch } from './client'

export interface EmbedTokenResponse {
  id: string
  asset_id: string
  public_url: string
  thumb_url: string
  created_at: string
  revoked: boolean
}

/** Public asset embed token endpoints — durable unauthenticated links to an asset's current version. */
export const embedTokenApi = {
  /** GET /api/v1/assets/:id/embed-token — returns the active token, or null if none exists (404). */
  get: (assetId: string): Promise<EmbedTokenResponse | null> =>
    apiFetch<EmbedTokenResponse>(`/api/v1/assets/${assetId}/embed-token`).catch(
      (err: unknown) => {
        if (err instanceof ApiError && err.status === 404) return null
        throw err
      }
    ),

  /** POST /api/v1/assets/:id/embed-token (editor+) — idempotent: creates or returns the active token. */
  create: (assetId: string): Promise<EmbedTokenResponse> =>
    apiFetch<EmbedTokenResponse>(`/api/v1/assets/${assetId}/embed-token`, {
      method: 'POST',
    }),

  /** DELETE /api/v1/assets/:id/embed-token (editor+) — revokes the active token. */
  revoke: (assetId: string): Promise<void> =>
    apiFetch<void>(`/api/v1/assets/${assetId}/embed-token`, {
      method: 'DELETE',
    }),
}
