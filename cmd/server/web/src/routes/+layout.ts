import { browser } from '$app/environment'
import { replaceState } from '$app/navigation'
import type { User, Workspace } from '$lib/api'
import { ApiError, workspaceApi } from '$lib/api'
import { redirect } from '@sveltejs/kit'
import type { LayoutLoad } from './$types'

const PUBLIC_PATHS = [
  '/login',
  '/register',
  '/forgot-password',
  '/reset-password',
  '/invite',
  '/s/',
  '/demo/',
  '/welcome',
  '/docs',
]

export const ssr = false

export const load: LayoutLoad = async ({
  url,
  fetch,
  depends,
}): Promise<{
  user?: User
  workspace?: Workspace
  role?: string
  totalAssetCount?: number
}> => {
  depends('app:workspace')

  let result: {
    user?: User
    workspace?: Workspace
    role?: string
    totalAssetCount?: number
  } = {}

  if (!browser) return result

  if (PUBLIC_PATHS.some((p) => url.pathname.startsWith(p))) {
    return result
  }

  workspaceApi.useFetch(fetch)

  try {
    const me = await workspaceApi.me()
    result = {
      user: me.user,
      workspace: me.workspace,
      role: me.role,
      totalAssetCount: me.total_asset_count,
    }
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      redirect(303, '/login')
    }
    throw e
  }

  const wsParam = url.searchParams.get('ws')
  if (wsParam && wsParam !== result.workspace?.id) {
    try {
      const switched = await workspaceApi.switch(wsParam)
      const refreshed = await workspaceApi.me().catch(() => null)
      result = {
        user: result.user,
        workspace: switched.workspace,
        role: switched.role,
        totalAssetCount: refreshed?.total_asset_count ?? result.totalAssetCount,
      }
      const clean = new URL(url)
      clean.searchParams.delete('ws')
      replaceState(clean.pathname + clean.search, {})
    } catch {
      // not a member of that workspace — stay in current one
    }
  }

  return result
}
