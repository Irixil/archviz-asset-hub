import { apiFetch } from './client'

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export interface ExportConfig {
  id: string
  workspace_id: string
  project_id: string
  label: string
  dest_type: 'sftp' | 'gdrive'
  versions: 'current' | 'all'
  include_variants: boolean
  schedule_type: 'manual' | 'after_quiet'
  quiet_minutes?: number
  enabled: boolean
  last_run_at?: string
  last_run_status?: 'ok' | 'partial' | 'failed' | 'pending'
  last_error?: string
  created_at: string
  updated_at: string
}

export interface ExportRun {
  id: string
  export_config_id: string
  triggered_by?: string
  status: 'pending' | 'running' | 'done' | 'failed'
  assets_total: number
  assets_exported: number
  assets_skipped: number
  bytes_written: number
  error?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

export const exportsApi = {
  list: () => apiFetch<ExportConfig[]>(`${API_BASE}/api/v1/exports`),

  create: (body: unknown) =>
    apiFetch<ExportConfig>(`${API_BASE}/api/v1/exports`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  get: (id: string) =>
    apiFetch<ExportConfig>(`${API_BASE}/api/v1/exports/${id}`),

  update: (id: string, body: unknown) =>
    apiFetch<ExportConfig>(`${API_BASE}/api/v1/exports/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  delete: (id: string) =>
    apiFetch<void>(`${API_BASE}/api/v1/exports/${id}`, { method: 'DELETE' }),

  validateDest: (body: unknown) =>
    apiFetch<{ ok: boolean }>(
      `${API_BASE}/api/v1/exports/validate-destination`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }
    ),

  trigger: (id: string) =>
    apiFetch<ExportRun>(`${API_BASE}/api/v1/exports/${id}/trigger`, {
      method: 'POST',
    }),

  getRun: (runId: string) =>
    apiFetch<ExportRun>(`${API_BASE}/api/v1/exports/runs/${runId}`),

  listRuns: (id: string, limit = 20, offset = 0) =>
    apiFetch<ExportRun[]>(
      `${API_BASE}/api/v1/exports/${id}/runs?limit=${limit}&offset=${offset}`
    ),
}
