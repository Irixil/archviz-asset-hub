import type { BulkFieldsPreviewResponse } from '$lib/api'
import { assetFieldApi, tagApi } from '$lib/api'
import { m } from '$lib/paraglide/messages'
import { assetsStore } from '$lib/stores/assets.svelte'
import type { Command } from './types'

export type TagEdit = { tag: string; mode: 'add' | 'remove' }
export type FieldEdit = {
  fieldId: string
  value: string | number | boolean | null // null = clear
}

export class BulkMetadataCommand implements Command {
  constructor(
    private assetIds: string[],
    private tagEdits: TagEdit[],
    private fieldEdits: FieldEdit[],
    private previewSnapshot: BulkFieldsPreviewResponse
  ) {}

  label() {
    const parts: string[] = []
    if (this.tagEdits.length)
      parts.push(
        m.cmd_bulk_metadata_tags({ count: String(this.tagEdits.length) })
      )
    if (this.fieldEdits.length)
      parts.push(
        m.cmd_bulk_metadata_fields({ count: String(this.fieldEdits.length) })
      )
    return m.cmd_bulk_metadata({
      changes: parts.join(', '),
      assets: String(this.assetIds.length),
    })
  }

  async apply() {
    await Promise.all(
      this.tagEdits.map((te) => tagApi.bulkTag(this.assetIds, te.tag, te.mode))
    )
    for (const te of this.tagEdits) {
      this.assetIds.forEach((id) =>
        te.mode === 'add'
          ? assetsStore.addTag(id, te.tag)
          : assetsStore.removeTag(id, te.tag)
      )
    }

    if (this.fieldEdits.length) {
      await assetFieldApi.bulkPatch(
        this.assetIds,
        this.fieldEdits.map((fe) => ({ field_id: fe.fieldId, value: fe.value }))
      )
      for (const id of this.assetIds) {
        assetsStore.patchFieldValues(
          id,
          this.fieldEdits.map((fe) => ({
            fieldId: fe.fieldId,
            value: fe.value,
          }))
        )
      }
    }
  }

  async revert() {
    await Promise.all(
      this.tagEdits.map((te) => {
        const inverse: 'add' | 'remove' = te.mode === 'add' ? 'remove' : 'add'
        return tagApi.bulkTag(this.assetIds, te.tag, inverse)
      })
    )
    for (const te of this.tagEdits) {
      const inverse: 'add' | 'remove' = te.mode === 'add' ? 'remove' : 'add'
      this.assetIds.forEach((id) =>
        inverse === 'add'
          ? assetsStore.addTag(id, te.tag)
          : assetsStore.removeTag(id, te.tag)
      )
    }

    // Best-effort: clear fields that were actually changed (full per-asset restore not available)
    if (this.fieldEdits.length) {
      const editedFieldIds = new Set(this.fieldEdits.map((e) => e.fieldId))
      const fieldsToRevert = this.previewSnapshot.fields.filter((f) =>
        editedFieldIds.has(f.field_id)
      )
      if (fieldsToRevert.length) {
        await assetFieldApi.bulkPatch(
          this.assetIds,
          fieldsToRevert.map((f) => ({ field_id: f.field_id, value: null }))
        )
        for (const id of this.assetIds) {
          assetsStore.patchFieldValues(
            id,
            fieldsToRevert.map((f) => ({ fieldId: f.field_id, value: null }))
          )
        }
      }
    }
  }

  rollback() {
    for (const te of this.tagEdits) {
      const inverse: 'add' | 'remove' = te.mode === 'add' ? 'remove' : 'add'
      this.assetIds.forEach((id) =>
        inverse === 'add'
          ? assetsStore.addTag(id, te.tag)
          : assetsStore.removeTag(id, te.tag)
      )
    }
    if (this.fieldEdits.length) {
      const editedFieldIds = new Set(this.fieldEdits.map((fe) => fe.fieldId))
      const fieldsToRevert = this.previewSnapshot.fields.filter((f) =>
        editedFieldIds.has(f.field_id)
      )
      if (fieldsToRevert.length) {
        this.assetIds.forEach((id) =>
          assetsStore.patchFieldValues(
            id,
            fieldsToRevert.map((f) => ({ fieldId: f.field_id, value: null }))
          )
        )
      }
    }
  }
}
