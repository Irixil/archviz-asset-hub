import { projectApi, type Project } from '$lib/api'
import { navigationStore } from './navigation.svelte'

let projects = $state<Project[]>([])
let stale = $state(false)

const projectAssetCount = $derived(
  projects.reduce((sum, p) => sum + p.asset_count, 0)
)
const activeProjectName = $derived(
  navigationStore.activeProjectId
    ? (projects.find((p) => p.id === navigationStore.activeProjectId)?.name ??
        '项目')
    : null
)

export const projectsStore = {
  get projects() {
    return projects
  },
  get projectAssetCount() {
    return projectAssetCount
  },
  get activeProjectName() {
    return activeProjectName
  },
  get stale() {
    return stale
  },

  invalidate() {
    stale = true
  },

  async load() {
    stale = false
    try {
      projects = await projectApi.list()
    } catch {
      // silently ignore — 401 redirect handled by api client
    }
  },

  async create(data: { name: string; color: string }) {
    await projectApi.create(data)
    await projectsStore.load()
  },

  async update(
    id: string,
    data: {
      name?: string
      cover_asset_id?: string
    }
  ) {
    await projectApi.update(id, data)
    await projectsStore.load()
  },

  async delete(id: string) {
    await projectApi.delete(id)
    if (navigationStore.activeProjectId === id) navigationStore.clear()
    await projectsStore.load()
  },
}
