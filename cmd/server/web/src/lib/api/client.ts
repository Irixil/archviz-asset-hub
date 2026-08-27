import type { definitions } from './types.gen'
import type { AuthResponse } from './workspace'

export type Config = definitions['api.ConfigResponse']
export type Variant = definitions['api.VariantResponse']
export type ListVariantsResponse = definitions['api.ListVariantsResponse']
export type CreateVariantResponse = definitions['api.CreateVariantResponse']
export type CoveringWorkflow = definitions['service.CoveringWorkflowDTO']
export type IngressSource = definitions['api.IngressSourceResponse']
export type IngressLogEntry = definitions['api.IngressLogResponse']
export type IngressRule = definitions['api.IngressRuleResponse']
export type AutomateVariantsResponse =
  definitions['api.AutomateVariantsResponse']
export type CreateIngressSourceParams =
  definitions['api.CreateIngressSourceReq']
export type UpdateIngressSourceParams =
  definitions['api.UpdateIngressSourceReq']
export type CreateIngressRuleParams = definitions['api.IngressRuleReq']
export type WatermarkAsset = definitions['api.WatermarkAssetResponse']
export type PromoteVariantResponse = definitions['api.PromoteVariantResponse']
export type SetVariantThumbnailResponse =
  definitions['api.SetVariantThumbnailResponse']
export type RerunVariantResponse = definitions['api.RerunVariantResponse']
export type ValidateCommandResponse = definitions['api.ValidateCommandResponse']
export type ParamHistoryEntry = definitions['api.ParamHistoryEntryResponse']
export type VariantParamHistoryResponse =
  definitions['api.VariantParamHistoryResponse']
export type AuditLogResponse = definitions['api.EventListResponse']
export type ActivityFeedResponse = definitions['api.ActivityFeedResponse']
export type AuditEvent = definitions['api.EventResponse']
export type ActivityEvent = definitions['api.ActivityEventResponse']

const API_BASE = import.meta.env.VITE_API_URL ?? ''

/**
 * Custom error class thrown by apiFetch when an API response is not ok.
 * Contains the HTTP status code and error message. `body` carries the parsed
 * JSON error payload when a caller needs structured details beyond the
 * message (e.g. the `duplicate_of` block on a 409 duplicate_content response).
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body?: unknown
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }
  if (res.status === 204) {
    // No content
    return {} as T
  }
  return res.json() as Promise<T>
}

/**
 * Like apiFetch but returns the raw Response without throwing or redirecting.
 * Use when you need to inspect status codes, read headers, or handle blobs.
 */
export async function apiFetchRaw(
  path: string,
  init: RequestInit = {}
): Promise<Response> {
  return fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })
}

/**
 * Base fetch wrapper that injects credentials and Content-Type headers.
 * Redirects to /login on 401 responses. Throws ApiError on non-ok responses.
 * @param path API path (relative to API_BASE)
 * @param init Fetch request options
 * @param fetch Optional custom fetch implementation (defaults to window.fetch)
 */
export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
  fetch = window.fetch
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })

  if (
    res.status === 401 &&
    typeof window !== 'undefined' &&
    path !== '/api/v1/workspace/me'
  ) {
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  return handleResponse<T>(res)
}

/** Server configuration — fetch public config flags (e.g. demo mode). GET /config (public) */
export const configApi = {
  load: (fetch?: typeof window.fetch) =>
    apiFetch<Config>('/config', undefined, fetch).catch(() => {
      console.warn('unable to load config')
      return {
        demo: false,
        mailHost: '',
      }
    }),
}

/** Authentication endpoints — register, login, token refresh. */
export const authApi = {
  /** POST /auth/register (public) — create a new account. */
  register: (name: string, email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),

  /** POST /auth/login (public) — authenticate with email and password. */
  login: (email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  /** POST /auth/refresh — refresh the JWT token (requires auth). */
  refresh: () =>
    apiFetch<{ token: string }>('/auth/refresh', { method: 'POST' }),

  forgotPassword: (email: string) =>
    apiFetch<void>('/auth/forgot-password', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  resetPassword: (token: string, password: string) =>
    apiFetch<void>('/auth/reset-password', {
      method: 'POST',
      body: JSON.stringify({ token, password }),
    }),

  changePassword: (currentPassword: string, newPassword: string) =>
    apiFetch<void>('/auth/password', {
      method: 'PATCH',
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    }),

  /** POST /demo/session (demo build only) — create a demo session. */
  demoSession: () =>
    apiFetch<{
      token: string
      workspace_id: string
      user_id: string
      is_demo: boolean
    }>('/demo/session', { method: 'POST' }),

  /** GET /auth/me — current user profile + linked identities. */
  me: () => apiFetch<MeResponse>('/auth/me'),

  updateMe: (displayName: string) =>
    apiFetch<MeResponse>('/api/v1/users/me', {
      method: 'PATCH',
      body: JSON.stringify({ display_name: displayName }),
    }),

  requestEmailChange: (email: string) =>
    apiFetch<{ pending_email: string }>('/api/v1/users/me/email', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  cancelPendingEmail: () =>
    apiFetch<void>('/api/v1/users/me/email/pending', { method: 'DELETE' }),

  deleteMe: (password = '') =>
    apiFetch<void>('/api/v1/users/me', {
      method: 'DELETE',
      body: JSON.stringify({ password }),
    }),

  uploadAvatar: async (file: File, fetch = window.fetch) => {
    const form = new FormData()
    form.append('avatar', file)
    const res = await fetch(`${API_BASE}/api/v1/users/me/avatar`, {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    return handleResponse<MeResponse>(res)
  },

  deleteAvatar: () =>
    apiFetch<void>('/api/v1/users/me/avatar', { method: 'DELETE' }),

  /** DELETE /auth/oidc/link — unlink OIDC identity. */
  unlinkOIDC: () => apiFetch('/auth/oidc/link', { method: 'DELETE' }),

  /** DELETE /auth/google/link — unlink Google identity. */
  unlinkGoogle: () => apiFetch('/auth/google/link', { method: 'DELETE' }),

  /** DELETE /auth/canva/link — unlink Canva identity. */
  unlinkCanva: () => apiFetch('/auth/canva/link', { method: 'DELETE' }),
}

export type MeResponse = definitions['api.MeResponse']

/**
 * Ingress (asset import from external sources) endpoints.
 * Includes: sources CRUD, rules, and event log.
 */
export const ingressApi = {
  // Sources
  /** GET /api/v1/ingress/sources — list all ingress sources in the workspace. */
  list: () => apiFetch<IngressSource[]>('/api/v1/ingress/sources'),

  /** GET /api/v1/ingress/sources/:id — fetch an ingress source by ID. */
  get: (id: string) => apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`),

  /** POST /api/v1/ingress/sources (editor+) — create a new ingress source. */
  create: (params: CreateIngressSourceParams) =>
    apiFetch<IngressSource>('/api/v1/ingress/sources', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/ingress/sources/:id (editor+) — update an ingress source config. */
  update: (id: string, params: UpdateIngressSourceParams) =>
    apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/ingress/sources/:id (owner only) — delete an ingress source. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${id}`, { method: 'DELETE' }),

  /** POST /api/v1/ingress/sources/:id/test (editor+) — test connection to source. */
  test: (id: string) =>
    apiFetch<{ ok: boolean }>(`/api/v1/ingress/sources/${id}/test`, {
      method: 'POST',
    }),

  /** POST /api/v1/ingress/sources/:id/poll (editor+) — manually trigger a poll job. */
  poll: (id: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/sources/${id}/poll`, {
      method: 'POST',
    }),

  // Rules
  /** GET /api/v1/ingress/sources/:id/rules — list rules for a source. */
  listRules: (sourceId: string) =>
    apiFetch<IngressRule[]>(`/api/v1/ingress/sources/${sourceId}/rules`),

  /** POST /api/v1/ingress/sources/:id/rules (editor+) — create an ingress rule. */
  createRule: (sourceId: string, params: CreateIngressRuleParams) =>
    apiFetch<IngressRule>(`/api/v1/ingress/sources/${sourceId}/rules`, {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/ingress/sources/:id/rules/:rid (editor+) — update an ingress rule. */
  updateRule: (
    sourceId: string,
    ruleId: string,
    params: Partial<CreateIngressRuleParams>
  ) =>
    apiFetch<IngressRule>(
      `/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`,
      {
        method: 'PUT',
        body: JSON.stringify(params),
      }
    ),

  /** DELETE /api/v1/ingress/sources/:id/rules/:rid (editor+) — delete an ingress rule. */
  deleteRule: (sourceId: string, ruleId: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`, {
      method: 'DELETE',
    }),

  /** PUT /api/v1/ingress/sources/:id/rules/reorder (editor+) — reorder rules by position. */
  reorderRules: (
    sourceId: string,
    entries: { id: string; position: number }[]
  ) =>
    apiFetch<IngressRule[]>(
      `/api/v1/ingress/sources/${sourceId}/rules/reorder`,
      {
        method: 'PUT',
        body: JSON.stringify(entries),
      }
    ),

  // Log
  /** GET /api/v1/ingress/sources/:id/log — list ingress log entries for a source (filter by status). */
  getSourceLog: (sourceId: string, status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(
      `/api/v1/ingress/sources/${sourceId}/log${qs}`
    )
  },

  /** GET /api/v1/ingress/log — list workspace-wide ingress log entries (filter by status). */
  getWorkspaceLog: (status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(`/api/v1/ingress/log${qs}`)
  },

  /** DELETE /api/v1/ingress/log/:id (editor+) — delete a log entry. */
  deleteLogEntry: (entryId: string) =>
    apiFetch<void>(`/api/v1/ingress/log/${entryId}`, { method: 'DELETE' }),

  /** POST /api/v1/ingress/log/:id/retry (editor+) — retry a failed ingestion job. */
  retryLogEntry: (entryId: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/log/${entryId}/retry`, {
      method: 'POST',
    }),
}

/** Variant management endpoints — list, create, upload, delete, file/preview URLs. */
export const variantApi = {
  /** GET /api/v1/assets/:id/variants — list all variants for an asset. */
  list: (assetId: string) =>
    apiFetch<ListVariantsResponse>(`/api/v1/assets/${assetId}/variants`),

  /** POST /api/v1/assets/:id/variants (editor+) — enqueue a variant generation job. */
  create: (assetId: string, type: string, params: object) =>
    apiFetch<CreateVariantResponse>(`/api/v1/assets/${assetId}/variants`, {
      method: 'POST',
      body: JSON.stringify({ type, params }),
    }),

  /** POST /api/v1/assets/:id/variants/automate (editor+) — create a disabled workflow from current variants. */
  automate: (
    assetId: string,
    scope: 'workspace' | 'project' | 'folder' | 'asset'
  ) =>
    apiFetch<AutomateVariantsResponse>(
      `/api/v1/assets/${assetId}/variants/automate`,
      {
        method: 'POST',
        body: JSON.stringify({ scope }),
      }
    ),

  /** GET /api/v1/assets/:id/variants/watermark — resolve the watermark asset selected by backend fallback rules. */
  resolveWatermark: (assetId: string) =>
    apiFetch<WatermarkAsset>(`/api/v1/assets/${assetId}/variants/watermark`),

  /** POST /api/v1/assets/:id/variants/upload (editor+) — upload a manually-created variant. */
  uploadManual: (assetId: string, file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return apiFetch<Variant>(`/api/v1/assets/${assetId}/variants/upload`, {
      method: 'POST',
      headers: {},
      body: fd,
    })
  },

  /** PATCH /api/v1/assets/:id/variants/:vid (editor+) — update editable fields on a variant. */
  patch: (assetId: string, variantId: string, body: { title: string }) =>
    apiFetch<Variant>(`/api/v1/assets/${assetId}/variants/${variantId}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  /** PUT /api/v1/assets/:id/variants/sharing (editor+) — bulk update variant sharing flags. */
  updateSharing: (assetId: string, updates: Record<string, boolean>) =>
    apiFetch<ListVariantsResponse>(
      `/api/v1/assets/${assetId}/variants/sharing`,
      {
        method: 'PUT',
        body: JSON.stringify({ updates }),
      }
    ),

  /** DELETE /api/v1/assets/:id/variants/:vid (editor+) — delete a variant. */
  delete: (assetId: string, variantId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/variants/${variantId}`, {
      method: 'DELETE',
    }),

  /** GET /api/v1/variants/validate-command — synchronously validate a custom_ffmpeg command. */
  validateCommand: (command: string) =>
    apiFetch<ValidateCommandResponse>(
      `/api/v1/variants/validate-command?q=${encodeURIComponent(command)}`
    ),

  /**
   * GET /api/v1/variant-param-history — distinct previous transform_params for a variant
   * type, most-recent first (workspace scope comes from the auth token). Never throws —
   * history is a convenience feature, so a failed fetch resolves to an empty list.
   */
  paramHistory: (variantType: string): Promise<readonly ParamHistoryEntry[]> =>
    apiFetch<VariantParamHistoryResponse>(
      `/api/v1/variant-param-history?type=${encodeURIComponent(variantType)}`
    )
      .then((res) => res.entries ?? [])
      .catch(() => []),

  /** POST /api/v1/assets/:id/variants/:vid/promote (editor+) — create a new asset from a variant. */
  promote: (assetId: string, variantId: string, name: string) =>
    apiFetch<PromoteVariantResponse>(
      `/api/v1/assets/${assetId}/variants/${variantId}/promote`,
      {
        method: 'POST',
        body: JSON.stringify({ name }),
      }
    ),

  /** POST /api/v1/assets/:id/variants/:vid/set-thumbnail (editor+) — set the variant file as the asset thumbnail source. */
  setThumbnail: (assetId: string, variantId: string) =>
    apiFetch<SetVariantThumbnailResponse>(
      `/api/v1/assets/${assetId}/variants/${variantId}/set-thumbnail`,
      { method: 'POST' }
    ),

  /** POST /api/v1/assets/:id/variants/:vid/rerun (editor+) — re-run a supported variant in place. */
  rerun: (
    assetId: string,
    variantId: string,
    params?: Record<string, unknown>
  ) =>
    apiFetch<RerunVariantResponse>(
      `/api/v1/assets/${assetId}/variants/${variantId}/rerun`,
      {
        method: 'POST',
        body: JSON.stringify({ params }),
      }
    ),

  /** GET /api/v1/assets/:id/variants/:vid/file — file URL for a variant. */
  fileUrl: (assetId: string, variantId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/variants/${variantId}/file`,

  /** GET /api/v1/assets/:id/variants/:vid/thumb — thumbnail URL for a variant. */
  thumbUrl: (assetId: string, variantId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/variants/${variantId}/thumb`,

  /** GET /api/v1/assets/:id/preview — in-memory cached preview transform URL (query params: w, h, fit, format, q). */
  previewUrl: (
    assetId: string,
    params: {
      w?: number
      h?: number
      fit?: string
      format?: string
      q?: number
    }
  ): string => {
    const qs = new URLSearchParams()
    if (params.w) qs.set('w', String(params.w))
    if (params.h) qs.set('h', String(params.h))
    if (params.fit) qs.set('fit', params.fit)
    if (params.format) qs.set('format', params.format)
    if (params.q) qs.set('q', String(params.q))
    return `${API_BASE}/api/v1/assets/${assetId}/preview?${qs.toString()}`
  },
}

/** Opens a Server-Sent Events stream for workspace-scoped real-time events (e.g. thumbnail_ready). */
export function openThumbnailEvents(): EventSource {
  return new EventSource(`${API_BASE}/api/v1/events`, { withCredentials: true })
}

/** Activity log and audit event endpoints — list asset/project/workspace events, export. */
export const activityApi = {
  /** GET /api/v1/assets/:id/events — list audit events for an asset (paginated). */
  listAssetEvents: (
    assetId: string,
    params: { limit?: number; cursor?: string; types?: string } = {}
  ) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<AuditLogResponse>(
      `/api/v1/assets/${assetId}/events${qs ? '?' + qs : ''}`
    )
  },

  /** GET /api/v1/projects/:id/events — list audit events for a project (paginated). */
  listProjectEvents: (
    projectId: string,
    params: { limit?: number; cursor?: string; types?: string } = {}
  ) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<AuditLogResponse>(
      `/api/v1/projects/${projectId}/events${qs ? '?' + qs : ''}`
    )
  },

  /** GET /api/v1/activity — list workspace-wide activity feed (paginated, filterable by user and event type). */
  listWorkspaceActivity: (
    params: {
      limit?: number
      cursor?: string
      types?: string
      user_id?: string
    } = {}
  ) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    if (params.user_id) q.set('user_id', params.user_id)
    const qs = q.toString()
    return apiFetch<ActivityFeedResponse>(
      `/api/v1/activity${qs ? '?' + qs : ''}`
    )
  },

  /** GET /api/v1/activity/export — export workspace activity as CSV (optionally filtered by date range). */
  exportCSV: (params: { since?: string; until?: string } = {}) => {
    const q = new URLSearchParams({ format: 'csv' })
    if (params.since) q.set('since', params.since)
    if (params.until) q.set('until', params.until)
    return `${API_BASE}/api/v1/activity/export?${q.toString()}`
  },
}

/** Stack endpoints — export as ZIP or merge into GIF/PDF. */
export const stackApi = {
  /** POST /api/v1/stack/export — download stacked assets as a ZIP file. */
  exportZip: async (
    assetIds: string[],
    filename = 'stack-export',
    variantMode?: 'none' | 'shared' | 'all'
  ): Promise<void> => {
    const res = await apiFetchRaw('/api/v1/stack/export', {
      method: 'POST',
      body: JSON.stringify({
        asset_ids: assetIds,
        filename,
        variant_mode: variantMode,
      }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(body.error ?? res.statusText)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${filename}.zip`
    a.click()
    URL.revokeObjectURL(url)
  },

  /** POST /api/v1/stack/merge — enqueue a GIF or PDF merge job. Returns job_id. */
  merge: async (
    assetIds: string[],
    outputType: 'gif' | 'pdf',
    filename = 'stack-merge',
    gifFrameMs = 500
  ): Promise<string> => {
    const res = await apiFetch<{ job_id: string }>('/api/v1/stack/merge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        asset_ids: assetIds,
        output_type: outputType,
        filename,
        gif_frame_ms: gifFrameMs,
      }),
    })
    return res.job_id
  },
}

export type OAuthConnection = definitions['api.ConnectionResponse']

export const integrationsApi = {
  list: (): Promise<OAuthConnection[]> =>
    apiFetch('/api/v1/integrations/connections'),
  disconnect: (id: string): Promise<void> =>
    apiFetch(`/api/v1/integrations/connections/${id}`, { method: 'DELETE' }),
}

/**
 * Formats a byte count as a human-readable string.
 * @param bytes Number of bytes
 * @returns Formatted string (e.g., "1.5 MB")
 */
export function formatBytes(bytes: number, fractionDigits = 1): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(fractionDigits)} MB`
}

export { mimeCategory, type MimeCategory } from '$lib/utils/mime'
