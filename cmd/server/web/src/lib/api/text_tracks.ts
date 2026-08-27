import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type TextTrack = definitions['api.TextTrackResponse']
export type TextTracksResponse = definitions['api.ListTextTracksResponse']
export type CreateTextTrackResponse = definitions['api.CreateTextTrackResponse']

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export const textTrackApi = {
  list: async (assetId: string): Promise<TextTrack[]> => {
    const res = await apiFetch<TextTracksResponse>(
      `/api/v1/assets/${assetId}/text-tracks`
    )
    return [...res.text_tracks]
  },

  get: (assetId: string, trackId: string): Promise<TextTrack> =>
    apiFetch<TextTrack>(`/api/v1/assets/${assetId}/text-tracks/${trackId}`),

  create: (
    assetId: string,
    body: { source: string; lang?: string; params?: Record<string, unknown> }
  ): Promise<TextTrack> =>
    apiFetch<CreateTextTrackResponse>(`/api/v1/assets/${assetId}/text-tracks`, {
      method: 'POST',
      body: JSON.stringify(body),
    }).then((res) => res.text_track),

  delete: (assetId: string, trackId: string): Promise<void> =>
    apiFetch<void>(`/api/v1/assets/${assetId}/text-tracks/${trackId}`, {
      method: 'DELETE',
    }),

  downloadUrl: (assetId: string, trackId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/text-tracks/${trackId}/download`,
}
