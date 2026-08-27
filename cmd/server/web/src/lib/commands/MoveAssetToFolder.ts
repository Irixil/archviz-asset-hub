import { assetApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { foldersStore } from '$lib/stores/folders.svelte'
import type { Command } from './types'

export class MoveAssetToFolder implements Command {
  constructor(
    private assetId: string,
    private before: string | null,
    private beforeName: string | null,
    private after: string | null,
    private afterName: string | null,
    private projectId: string | null = null
  ) {}

  label() {
    return m.cmd_move_folder({ to: this.afterName ?? '—' })
  }

  async apply() {
    await assetApi.updateFolder(this.assetId, this.after)
    assetsStore.patchAsset(this.assetId, { folder_id: this.after })
    if (this.projectId) foldersStore.invalidateForProject(this.projectId)
  }

  async revert() {
    await assetApi.updateFolder(this.assetId, this.before)
    assetsStore.patchAsset(this.assetId, { folder_id: this.before })
    if (this.projectId) foldersStore.invalidateForProject(this.projectId)
  }

  rollback() {
    assetsStore.patchAsset(this.assetId, { folder_id: this.before })
  }
}
