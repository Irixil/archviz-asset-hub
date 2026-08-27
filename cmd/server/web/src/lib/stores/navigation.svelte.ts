import type { ArchiveGroupId } from '$lib/data/architectureTaxonomy'
import { browserDetectStore } from './browserDetect.svelte'

let activeProjectId = $state<string | null>(null)
let activeFolderId = $state<string | null>(null)
let activeCollectionId = $state<string | null>(null)
let activeArchiveGroupId = $state<ArchiveGroupId | null>('assets')
let sidebarVisible = $state(browserDetectStore.onWideDevice)

export const navigationStore = {
  get activeProjectId() {
    return activeProjectId
  },
  get activeFolderId() {
    return activeFolderId
  },
  get activeCollectionId() {
    return activeCollectionId
  },
  get activeArchiveGroupId() {
    return activeArchiveGroupId
  },
  get sidebarVisible() {
    return sidebarVisible
  },
  set sidebarVisible(v) {
    sidebarVisible = v
  },

  toggleSidebarVisible() {
    sidebarVisible = !sidebarVisible
  },

  selectProject(id: string | null) {
    activeProjectId = id
    activeFolderId = null
    activeCollectionId = null
    activeArchiveGroupId = id ? null : 'assets'
  },

  selectFolder(folderId: string | null) {
    activeFolderId = folderId
    activeCollectionId = null
  },

  selectCollection(id: string | null) {
    activeCollectionId = id
    activeProjectId = null
    activeFolderId = null
    activeArchiveGroupId = null
  },

  selectArchiveGroup(id: ArchiveGroupId | null) {
    activeArchiveGroupId = id
    activeProjectId = null
    activeFolderId = null
    activeCollectionId = null
  },

  clear() {
    activeProjectId = null
    activeFolderId = null
    activeCollectionId = null
    activeArchiveGroupId = 'assets'
  },
}
