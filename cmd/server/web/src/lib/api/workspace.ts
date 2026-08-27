import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Workspace = definitions['api.WorkspaceResponse']
export type WorkspaceMeResponse = definitions['api.WorkspaceMeResponse']
export type WorkspaceMember = definitions['api.MemberResponse']
export type WorkspaceInvite = definitions['api.InviteResponse']
export type AuthResponse = definitions['api.AuthResponse']
export type WorkspaceWithRole = definitions['api.WorkspaceWithRoleResponse']
export type SwitchWorkspaceResponse = definitions['api.SwitchWorkspaceResponse']
export type User = definitions['api.UserResponse']

export const workspaceApi = {
  fetch: window.fetch,
  useFetch: (
    f: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
  ) => (workspaceApi.fetch = f),

  /** GET /api/v1/workspace/me — fetch current workspace info. */
  me: () =>
    apiFetch<WorkspaceMeResponse>(
      '/api/v1/workspace/me',
      undefined,
      workspaceApi.fetch
    ),

  /** PUT /api/v1/workspace/settings (owner only) — update workspace settings. */
  updateSettings: (settings: {
    version_retention_count?: number
    exif_keep?: boolean
    exif_keep_gps?: boolean
    locked_taxonomy?: boolean
    auto_tag_enabled?: boolean
    auto_tag_mode?: 'pending' | 'silent'
    duplicate_detection_mode?: 'off' | 'warn' | 'block'
  }) =>
    apiFetch<Workspace>('/api/v1/workspace/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  /** POST /api/v1/workspace/jobs/:type/trigger (owner only) — trigger a predefined background job. */
  triggerJob: (type: string) =>
    apiFetch<{ enqueued: number }>(`/api/v1/workspace/jobs/${type}/trigger`, {
      method: 'POST',
    }),

  /** GET /api/v1/workspaces — list all workspaces the user is a member of. */
  list: () => apiFetch<WorkspaceWithRole[]>('/api/v1/workspaces'),

  /** POST /api/v1/workspace/switch — switch to a different workspace. */
  switch: (workspaceId: string) =>
    apiFetch<SwitchWorkspaceResponse>('/api/v1/workspace/switch', {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceId }),
    }),

  /** POST /api/v1/workspace (editor+) — create a new workspace. */
  createWorkspace: (name: string) =>
    apiFetch<AuthResponse>('/api/v1/workspace', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  /** POST /api/v1/workspace/invites (owner only) — send a workspace invitation. */
  createInvite: (email: string, role: 'editor' | 'viewer' = 'editor') =>
    apiFetch<{ id: string; invite_token: string; email: string; role: string }>(
      '/api/v1/workspace/invites',
      { method: 'POST', body: JSON.stringify({ email, role }) }
    ),

  /** POST /auth/invite/accept (public) — accept a workspace invitation. */
  acceptInvite: (token: string, name: string, password: string) =>
    apiFetch<AuthResponse>('/auth/invite/accept', {
      method: 'POST',
      body: JSON.stringify({ token, name, password }),
    }),

  /** GET /api/v1/workspace/members (owner only) — list workspace members. */
  listMembers: () => apiFetch<WorkspaceMember[]>('/api/v1/workspace/members'),

  /** DELETE /api/v1/workspace/members/:userId (owner only) — remove a member. */
  removeMember: (userId: string) =>
    apiFetch<void>(`/api/v1/workspace/members/${userId}`, { method: 'DELETE' }),

  /** PUT /api/v1/workspace/members/:userId (owner only) — change member role. */
  updateMemberRole: (userId: string, role: string) =>
    apiFetch<void>(`/api/v1/workspace/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),

  /** GET /api/v1/workspace/invites (owner only) — list pending invites. */
  listInvites: () => apiFetch<WorkspaceInvite[]>('/api/v1/workspace/invites'),

  /** DELETE /api/v1/workspace/invites/:inviteId (owner only) — cancel an invite. */
  deleteInvite: (inviteId: string) =>
    apiFetch<void>(`/api/v1/workspace/invites/${inviteId}`, {
      method: 'DELETE',
    }),
}
