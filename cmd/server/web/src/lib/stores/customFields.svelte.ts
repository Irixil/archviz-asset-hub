import type { AssetFieldValue, FieldDefinition, FieldScope } from '$lib/api'
import { fieldDefinitionApi } from '$lib/api'

class CustomFieldsStore {
  assetFields = $state<FieldDefinition[]>([])
  projectFields = $state<FieldDefinition[]>([])
  loading = $state(false)
  /** Per-asset field value cache: assetId → AssetFieldValue[]. Updated by patchField/clearField. */
  private fieldValues = $state<Map<string, AssetFieldValue[]>>(new Map())

  getFieldValues(assetId: string): AssetFieldValue[] | undefined {
    return this.fieldValues.get(assetId)
  }

  patchField(
    assetId: string,
    fieldId: string,
    value: string | number | boolean | null
  ) {
    const current = this.fieldValues.get(assetId)
    if (!current) return
    const updated = current.map((v) =>
      v.field_id === fieldId ? { ...v, value } : v
    )
    this.fieldValues = new Map(this.fieldValues).set(assetId, updated)
  }

  clearField(assetId: string, fieldId: string) {
    this.patchField(assetId, fieldId, null)
  }

  setFieldValues(assetId: string, values: readonly AssetFieldValue[]) {
    this.fieldValues = new Map(this.fieldValues).set(assetId, [...values])
  }

  async load(scope: FieldScope) {
    this.loading = true
    try {
      const defs = await fieldDefinitionApi.list(scope)
      if (scope === 'asset') {
        this.assetFields = [...defs]
      } else {
        this.projectFields = [...defs]
      }
    } finally {
      this.loading = false
    }
  }

  async loadBoth() {
    this.loading = true
    try {
      const [asset, project] = await Promise.all([
        fieldDefinitionApi.list('asset'),
        fieldDefinitionApi.list('project'),
      ])
      this.assetFields = [...asset]
      this.projectFields = [...project]
    } finally {
      this.loading = false
    }
  }

  fields(scope: FieldScope): FieldDefinition[] {
    return scope === 'asset' ? this.assetFields : this.projectFields
  }

  async reorder(scope: FieldScope, ordered: FieldDefinition[]) {
    const entries = ordered.map((f, i) => ({ id: f.id, position: i }))
    // Optimistic update
    if (scope === 'asset') {
      this.assetFields = ordered
    } else {
      this.projectFields = ordered
    }
    await fieldDefinitionApi.reorder(entries)
  }

  upsertLocal(scope: FieldScope, def: FieldDefinition) {
    const list = scope === 'asset' ? this.assetFields : this.projectFields
    const idx = list.findIndex((f) => f.id === def.id)
    const updated = idx >= 0 ? list.with(idx, def) : [...list, def]
    if (scope === 'asset') {
      this.assetFields = updated
    } else {
      this.projectFields = updated
    }
  }

  removeLocal(scope: FieldScope, id: string) {
    if (scope === 'asset') {
      this.assetFields = this.assetFields.filter((f) => f.id !== id)
    } else {
      this.projectFields = this.projectFields.filter((f) => f.id !== id)
    }
  }
}

export const customFieldsStore = new CustomFieldsStore()
