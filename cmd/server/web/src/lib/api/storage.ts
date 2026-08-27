import { apiFetch } from './client'

export interface AssetTypeBucket {
  image: number
  video: number
  audio: number
  document: number
  other: number
}

export interface ProjectStorageUsage {
  project_id: string | null
  project_name: string
  versions_bytes: number
  variants_bytes: number
  total_bytes: number
  folder_count: number
  by_type: AssetTypeBucket
}

export interface WorkspaceStorageUsage {
  versions_bytes: number
  variants_bytes: number
  total_bytes: number
  limit_bytes: number | null
  computed_at: string
  by_project: ProjectStorageUsage[]
  by_type: AssetTypeBucket
}

export interface FolderStorageUsage {
  folder_id: string | null
  folder_name: string
  versions_bytes: number
  variants_bytes: number
  total_bytes: number
}

export interface ProjectFolderStorageResponse {
  project_id: string
  folders: FolderStorageUsage[]
}

export function fetchWorkspaceStorage(): Promise<WorkspaceStorageUsage> {
  return apiFetch<WorkspaceStorageUsage>('/api/v1/workspace/storage')
}

export function fetchProjectFolderStorage(
  projectId: string
): Promise<ProjectFolderStorageResponse> {
  return apiFetch<ProjectFolderStorageResponse>(
    `/api/v1/workspace/storage/projects/${projectId}/folders`
  )
}
