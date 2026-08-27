import { ApiError, apiFetch } from './client'
import type { definitions } from './types.gen'

export type AssetVersion = definitions['api.VersionResponse']
export type UploadVersionResponse = definitions['api.VersionWithAssetResponse']
export type RestoreVersionResponse = definitions['api.VersionWithAssetResponse']

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export const versionApi = {
  /** GET /api/v1/assets/:id/versions — list all versions of an asset. */
  list: (assetId: string) =>
    apiFetch<AssetVersion[]>(`/api/v1/assets/${assetId}/versions`),

  /** POST /api/v1/assets/:id/versions (editor+) — upload a new version via XHR (with progress). */
  upload(
    assetId: string,
    file: File,
    comment: string,
    onProgress?: (pct: number) => void
  ): Promise<UploadVersionResponse> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${API_BASE}/api/v1/assets/${assetId}/versions`)
      xhr.withCredentials = true
      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable)
            onProgress(Math.round((e.loaded / e.total) * 100))
        })
      }
      xhr.addEventListener('load', () => {
        if (xhr.status === 201) {
          resolve(JSON.parse(xhr.responseText) as UploadVersionResponse)
        } else if (xhr.status === 401 && typeof window !== 'undefined') {
          window.location.href = '/login'
          reject(new ApiError(401, 'Unauthorized'))
        } else {
          const body = JSON.parse(xhr.responseText) as { error?: string }
          reject(new ApiError(xhr.status, body.error ?? xhr.statusText))
        }
      })
      xhr.addEventListener('error', () =>
        reject(new ApiError(0, 'Network error'))
      )
      const fd = new FormData()
      fd.append('file', file)
      if (comment) fd.append('comment', comment)
      xhr.send(fd)
    })
  },

  /** POST /api/v1/assets/:id/versions/:vid/restore (editor+) — restore a previous version as current. */
  restore: (assetId: string, versionId: string) =>
    apiFetch<RestoreVersionResponse>(
      `/api/v1/assets/${assetId}/versions/${versionId}/restore`,
      { method: 'POST' }
    ),

  /** DELETE /api/v1/assets/:id/versions/:vid (owner only) — delete a version. */
  delete: (assetId: string, versionId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/versions/${versionId}`, {
      method: 'DELETE',
    }),

  /** GET /api/v1/assets/:id/versions/:vid/file — file URL for a specific version. */
  fileUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/file`,

  /** GET /api/v1/assets/:id/versions/:vid/thumb — thumbnail URL for a specific version. */
  thumbUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/thumb`,
}
