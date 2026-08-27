import type { User, Workspace } from '$lib/api'

export interface AuthState {
  user: User | null
  workspace: Workspace | null
  role: string | null
  isAuthenticated: boolean
  totalAssetCount: number
}

const state = $state<AuthState>({
  user: null,
  workspace: null,
  role: null,
  isAuthenticated: false,
  totalAssetCount: 0,
})

export const authStore = {
  get user() {
    return state.user
  },
  get workspace() {
    return state.workspace
  },
  get role() {
    return state.role
  },
  get isAuthenticated() {
    return state.isAuthenticated
  },
  get totalAssetCount() {
    return state.totalAssetCount
  },
  /** True when the workspace has locked_taxonomy enabled. */
  get taxonomyLocked() {
    return state.workspace?.locked_taxonomy ?? false
  },
  /** True when the current user is allowed to create new tags. */
  get canCreateTag() {
    return (
      !(state.workspace?.locked_taxonomy ?? false) || state.role === 'owner'
    )
  },

  login(user: User, workspace: Workspace, role: string, totalAssetCount = 0) {
    state.user = user
    state.workspace = workspace
    state.role = role
    state.isAuthenticated = true
    state.totalAssetCount = totalAssetCount
  },

  setTotalAssetCount(count: number) {
    state.totalAssetCount = count
  },

  switchWorkspace(workspace: Workspace, role: string) {
    state.workspace = workspace
    state.role = role
  },

  patchWorkspace(patch: Partial<Workspace>) {
    if (state.workspace) state.workspace = { ...state.workspace, ...patch }
  },

  logout() {
    state.user = null
    state.workspace = null
    state.role = null
    state.isAuthenticated = false
  },
}
