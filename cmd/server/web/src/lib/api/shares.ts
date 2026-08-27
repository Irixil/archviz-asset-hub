import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Share = definitions['api.ShareResponse']
export type PublicShare = definitions['api.PublicShareView']
export type ShareAssetsResponse = definitions['api.ShareAssetsResponse']
export type ShareInfoResponse = definitions['api.ShareInfoResponse']
export type ShareAccessResponse = definitions['api.ShareAccessResponse']
export type CreateShareParams = definitions['api.CreateShareRequest']
export type UpdateShareParams = definitions['api.UpdateShareRequest']

const API_BASE = import.meta.env.VITE_API_URL ?? ''

function shareHeaders(token?: string | null): Record<string, string> {
  return token ? { 'X-Share-Token': token } : {}
}

/**
 * Fetch wrapper for public /shared/:id/* endpoints.
 * No credentials cookie, no 401 redirect. Returns raw Response for callers
 * that need status inspection, header reading, or blob downloads.
 */
export function publicShareFetch(
  url: string,
  init: RequestInit = {},
  token?: string | null
): Promise<Response> {
  return fetch(url, {
    ...init,
    headers: { ...shareHeaders(token), ...init.headers },
  })
}

/** Public share endpoints — no auth cookie required, token via X-Share-Token header. */
export const publicShareApi = {
  /** GET /shared/:id/access — fetch share info (label, has_password). */
  getAccess: (shareId: string): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/access`),

  /** POST /shared/:id/access — submit visitor name + password, get session token. */
  postAccess: (
    shareId: string,
    visitorName: string,
    password: string
  ): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/access`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ visitor_name: visitorName, password }),
    }),

  /** GET /shared/:id/assets — list share assets (requires token for password-protected shares). */
  getAssets: (shareId: string, token?: string | null): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/assets`, {
      headers: shareHeaders(token),
    }),

  /** GET /shared/:id/comments?asset_id=... — list comments for an asset in the share. */
  getComments: (
    shareId: string,
    assetId: string,
    token?: string | null
  ): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/comments?asset_id=${assetId}`, {
      headers: shareHeaders(token),
    }),

  /** POST /shared/:id/comments — post a comment on a share asset. */
  postComment: (
    shareId: string,
    body: {
      asset_id?: string
      author_name?: string
      author_email?: string
      body: string
    },
    token?: string | null
  ): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/comments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...shareHeaders(token) },
      body: JSON.stringify(body),
    }),

  /** GET /shared/:id/export — download all share assets as ZIP. */
  downloadExport: (shareId: string, token?: string | null): Promise<Response> =>
    fetch(`${API_BASE}/shared/${shareId}/export`, {
      headers: shareHeaders(token),
    }),

  /** Fetch a file URL with the share token injected (for variant file downloads). */
  downloadFile: (url: string, token?: string | null): Promise<Response> =>
    fetch(url, { headers: shareHeaders(token) }),
}

export const shareApi = {
  /** GET /api/v1/shares — list all shares in the workspace. */
  list: () => apiFetch<Share[]>('/api/v1/shares'),

  /** GET /api/v1/shares/:id — fetch a share by ID. */
  get: (id: string) => apiFetch<Share>(`/api/v1/shares/${id}`),

  /** POST /api/v1/shares — create a new share link for assets. */
  create: (params: CreateShareParams) =>
    apiFetch<Share>('/api/v1/shares', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/shares/:id — update share settings (name, expiry, password). */
  update: (id: string, params: UpdateShareParams) =>
    apiFetch<Share>(`/api/v1/shares/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/shares/:id — revoke a share link. */
  revoke: (id: string) =>
    apiFetch<void>(`/api/v1/shares/${id}`, { method: 'DELETE' }),
}
