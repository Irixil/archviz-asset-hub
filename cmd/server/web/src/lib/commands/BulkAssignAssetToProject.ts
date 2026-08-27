import { assetApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { foldersStore } from '$lib/stores/folders.svelte'
import { projectsStore } from '$lib/stores/projects.svelte'
import type { Command } from './types'

export class BulkAssignAssetToProject implements Command {
  constructor(
    private assetIds: string[],
    private beforeProjectIds: Map<string, string | null>,
    private afterProjectId: string,
    private afterProjectName: string | null
  ) {}

  label() {
    return m.cmd_bulk_assign_project({
      count: String(this.assetIds.length),
      project: this.afterProjectName ?? '—',
    })
  }

  async apply() {
    await assetApi.bulkProject(this.assetIds, this.afterProjectId)
    for (const id of this.assetIds) {
      assetsStore.patchAsset(id, { project_id: this.afterProjectId })
    }
    assetsStore.invalidate()
    projectsStore.invalidate()
    foldersStore.invalidateForProject(this.afterProjectId)
  }

  async revert() {
    const groups = new Map<string | null, string[]>()
    for (const [assetId, prevId] of this.beforeProjectIds) {
      const key = prevId ?? null
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(assetId)
    }
    for (const [prevId, ids] of groups) {
      await assetApi.bulkProject(ids, prevId)
      for (const id of ids) {
        assetsStore.patchAsset(id, { project_id: prevId })
      }
    }
    assetsStore.invalidate()
    projectsStore.invalidate()
    foldersStore.invalidateForProject(this.afterProjectId)
    for (const prevId of new Set(this.beforeProjectIds.values())) {
      if (prevId) foldersStore.invalidateForProject(prevId)
    }
  }

  rollback() {
    for (const [id, prevId] of this.beforeProjectIds) {
      assetsStore.patchAsset(id, { project_id: prevId })
    }
  }
}
