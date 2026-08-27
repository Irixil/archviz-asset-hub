import { assetApi, folderApi, type Folder } from '$lib/api'
import { navigationStore } from './navigation.svelte'

let foldersByProject = $state<Record<string, Folder[]>>({})
let staleProjects = $state(new Set<string>())

const foldersForActiveProject = $derived(
  navigationStore.activeProjectId
    ? (foldersByProject[navigationStore.activeProjectId] ?? [])
    : []
)

export const foldersStore = {
  get foldersByProject() {
    return foldersByProject
  },
  get foldersForActiveProject() {
    return foldersForActiveProject
  },
  get staleProjects() {
    return staleProjects
  },

  invalidateForProject(projectId: string) {
    if (!(projectId in foldersByProject)) return
    staleProjects = new Set([...staleProjects, projectId])
  },

  async loadForProject(projectId: string) {
    staleProjects = new Set([...staleProjects].filter((id) => id !== projectId))
    try {
      const data = await folderApi.list(projectId)
      foldersByProject = { ...foldersByProject, [projectId]: data }
    } catch {
      // silently ignore
    }
  },

  async create(projectId: string, data: { name: string; parent_id?: string }) {
    await folderApi.create(projectId, data)
    await foldersStore.loadForProject(projectId)
  },

  async rename(folderId: string, projectId: string, name: string) {
    await folderApi.update(folderId, { name })
    await foldersStore.loadForProject(projectId)
  },

  async delete(folderId: string, projectId: string) {
    await folderApi.delete(folderId)
    await foldersStore.loadForProject(projectId)
  },

  async moveAssets(
    assetIds: string[],
    folderId: string | null,
    projectId: string
  ) {
    await Promise.all(assetIds.map((id) => assetApi.updateFolder(id, folderId)))
    await foldersStore.loadForProject(projectId)
  },
}
