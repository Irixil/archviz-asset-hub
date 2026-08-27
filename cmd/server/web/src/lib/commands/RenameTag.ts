import { m } from '$lib/paraglide/messages'
import { tagsManagementStore } from '$lib/stores/tagsManagement.svelte'
import type { Command } from './types'

export class RenameTag implements Command {
  constructor(
    private before: string,
    private after: string
  ) {}

  label() {
    return m.cmd_rename_tag({ name: this.after })
  }

  async apply() {
    await tagsManagementStore.patchTag(this.before, { name: this.after })
  }

  async revert() {
    await tagsManagementStore.patchTag(this.after, { name: this.before })
  }
}
