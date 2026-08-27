import { assetApi, type Asset } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { foldersStore } from '$lib/stores/folders.svelte'
import { projectsStore } from '$lib/stores/projects.svelte'
import type { Command } from './types'

type Membership = {
  project_id: string | null | undefined
  folder_id: string | null | undefined
  assetIds: string[]
}

export class BulkAssignAssetToFolder implements Command {
  private beforeProjectIds: Record<string, string | null>
  private assetIds: string[]
  private assetIdsByFolder: Membership[]

  constructor(
    assets: Asset[],
    private afterFolderId: string | null,
    private afterFolderName: string | null,
    private afterProjectId: string
  ) {
    this.assetIds = assets.map((a) => a.id)
    this.beforeProjectIds = assets.reduce(
      (acc, a) => ({ ...acc, [a.id]: a.project_id }),
      {}
    )
    this.assetIdsByFolder = assets.reduce((acc, a) => {
      const existing = acc.find(
        (b) =>
          (b.project_id ?? null) === (a.project_id ?? null) &&
          (b.folder_id ?? null) === (a.folder_id ?? null)
      )
      if (existing) {
        existing.assetIds.push(a.id)
      } else {
        acc.push({
          assetIds: [a.id],
          folder_id: a.folder_id,
          project_id: a.project_id,
        })
      }
      return acc
    }, [] as Membership[])
  }

  label() {
    return m.cmd_bulk_assign_folder({
      count: String(this.assetIds.length),
      folder: this.afterFolderName ?? '—',
    })
  }

  async apply() {
    const invalidatedProjects = new Set<string | null>()
    await foldersStore.moveAssets(
      this.assetIds,
      this.afterFolderId,
      this.afterProjectId
    )
    for (const id of this.assetIds) {
      assetsStore.patchAsset(id, {
        project_id: this.afterProjectId,
        folder_id: this.afterFolderId,
      })
      invalidatedProjects.add(this.beforeProjectIds[id])
    }
    ;[...invalidatedProjects]
      .filter(Boolean)
      .forEach((pid) => foldersStore.invalidateForProject(pid as string))
    assetsStore.invalidate()
    projectsStore.invalidate()
  }

  async revert() {
    const invalidatedProjects = new Set<string | null>()
    for (const before of this.assetIdsByFolder) {
      if (before.project_id) {
        await foldersStore.moveAssets(
          before.assetIds,
          before.folder_id ?? null,
          before.project_id
        )
        invalidatedProjects.add(before.project_id)
      } else {
        await assetApi.bulkProject(before.assetIds, before.project_id ?? null)
      }
      for (const id of before.assetIds) {
        assetsStore.patchAsset(id, { ...before })
      }
    }
    ;[...invalidatedProjects]
      .filter(Boolean)
      .forEach((pid) => foldersStore.invalidateForProject(pid as string))
    foldersStore.invalidateForProject(this.afterProjectId)
    projectsStore.invalidate()
    assetsStore.invalidate()
  }

  rollback() {
    for (const before of this.assetIdsByFolder) {
      for (const id of before.assetIds) {
        assetsStore.patchAsset(id, { ...before })
      }
    }
  }
}
