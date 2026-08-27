import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Folder = definitions['api.FolderResponse']

export const folderApi = {
  /** GET /api/v1/projects/:id/folders — list folders (hierarchy tree) in a project. */
  list: (projectId: string) =>
    apiFetch<Folder[]>(`/api/v1/projects/${projectId}/folders`),

  /** POST /api/v1/projects/:id/folders (editor+) — create a new folder. */
  create: (
    projectId: string,
    data: { name: string; parent_id?: string; position?: number }
  ) =>
    apiFetch<Folder>(`/api/v1/projects/${projectId}/folders`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /api/v1/folders/:id (editor+) — update folder name or position. */
  update: (id: string, data: { name?: string; position?: number }) =>
    apiFetch<Folder>(`/api/v1/folders/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /api/v1/folders/:id (owner only) — delete a folder. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/folders/${id}`, { method: 'DELETE' }),
}
