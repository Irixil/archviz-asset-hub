import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Project = definitions['api.ProjectResponse']

export const projectApi = {
  /** GET /api/v1/projects — list projects in current workspace. */
  list: () => apiFetch<Project[]>('/api/v1/projects'),

  /** GET /api/v1/projects/:id — fetch a project by ID. */
  get: (id: string) => apiFetch<Project>(`/api/v1/projects/${id}`),

  /** POST /api/v1/projects (editor+) — create a new project. */
  create: (data: { name: string; description?: string; color?: string }) =>
    apiFetch<Project>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /api/v1/projects/:id (editor+) — update project metadata. */
  update: (
    id: string,
    data: {
      name?: string
      description?: string
      color?: string
      cover_asset_id?: string
    }
  ) =>
    apiFetch<Project>(`/api/v1/projects/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /api/v1/projects/:id (owner only) — delete a project. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/projects/${id}`, { method: 'DELETE' }),
}
