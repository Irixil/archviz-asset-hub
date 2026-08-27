import { assetApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import { projectsStore } from '$lib/stores/projects.svelte'
import type { Command } from './types'

export class AssignAssetToProject implements Command {
  constructor(
    private assetId: string,
    private before: string | null,
    private beforeName: string | null,
    private after: string | null,
    private afterName: string | null
  ) {}

  label() {
    return this.after
      ? m.cmd_assign_project({ project: this.afterName ?? '—' })
      : m.cmd_unassign_project()
  }

  async apply() {
    await assetApi.bulkProject([this.assetId], this.after)
    assetsStore.patchAsset(this.assetId, { project_id: this.after })
    projectsStore.invalidate()
  }

  async revert() {
    await assetApi.bulkProject([this.assetId], this.before)
    assetsStore.patchAsset(this.assetId, { project_id: this.before })
    projectsStore.invalidate()
  }

  rollback() {
    assetsStore.patchAsset(this.assetId, { project_id: this.before })
  }
}
