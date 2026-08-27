import { apiFetch } from './client'
import type { FieldScope } from './models'
import type { definitions } from './types.gen'

export type FieldDefinition = definitions['api.FieldDefinitionResponse']
export type FieldDefinitionStats =
  definitions['api.FieldDefinitionStatsResponse']
export type AssetFieldsResponse = definitions['api.GetAssetFieldsResponse']
export type AssetFieldValue = definitions['api.assetFieldValueResponse']
export type BulkFieldPreviewEntry = definitions['api.BulkFieldPreviewEntry']
export type BulkFieldsPreviewResponse =
  definitions['api.BulkFieldsPreviewResponse']
export type BulkPatchAssetFieldsResponse =
  definitions['api.BulkPatchAssetFieldsResponse']
export type ProjectFieldsResponse = definitions['api.GetProjectFieldsResponse']
export type ProjectFieldValue = definitions['api.projectFieldValueResponse']

export const fieldDefinitionApi = {
  /** GET /api/v1/field-definitions — list field definitions by scope (workspace or project). */
  list: (scope: FieldScope) =>
    apiFetch<FieldDefinition[]>(`/api/v1/field-definitions?scope=${scope}`),

  /** GET /api/v1/field-definitions/:id — fetch a field definition by ID. */
  get: (id: string) =>
    apiFetch<FieldDefinition>(`/api/v1/field-definitions/${id}`),

  /** GET /api/v1/field-definitions/:id/stats — fetch usage stats for a field definition. */
  stats: (id: string) =>
    apiFetch<FieldDefinitionStats>(`/api/v1/field-definitions/${id}/stats`),

  /** POST /api/v1/field-definitions (editor+) — create a new field definition. */
  create: (params: {
    scope: FieldScope
    name: string
    key: string
    field_type: string
    options?: string | null
    required?: boolean
    position?: number
    inherit_from_project?: boolean
  }) =>
    apiFetch<FieldDefinition>('/api/v1/field-definitions', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/field-definitions/:id (editor+) — update a field definition. */
  update: (
    id: string,
    params: {
      name?: string
      options?: string | null
      required?: boolean
      position?: number
      inherit_from_project?: boolean
    }
  ) =>
    apiFetch<FieldDefinition>(`/api/v1/field-definitions/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/field-definitions/:id (editor+) — delete a field definition. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/field-definitions/${id}`, { method: 'DELETE' }),

  /** PUT /api/v1/field-definitions/reorder (editor+) — reorder field definitions. */
  reorder: (entries: { id: string; position: number }[]) =>
    apiFetch<void>('/api/v1/field-definitions/reorder', {
      method: 'PUT',
      body: JSON.stringify(entries),
    }),
}

export const assetFieldApi = {
  /** GET /api/v1/assets/:id/fields — fetch field values for an asset. */
  get: (assetId: string) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`),

  /** PATCH /api/v1/assets/:id/fields (editor+) — update field values for an asset. */
  patch: (
    assetId: string,
    values: { field_id: string; value: string | number | boolean | null }[]
  ) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),

  /** POST /api/v1/assets/bulk/fields/preview (editor+) — preview overwrite impact per field. */
  bulkPreview: (assetIds: string[], fieldIds: string[] = []) =>
    apiFetch<BulkFieldsPreviewResponse>('/api/v1/assets/bulk/fields/preview', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, field_ids: fieldIds }),
    }),

  /** PATCH /api/v1/assets/bulk/fields (editor+) — apply field values to multiple assets. */
  bulkPatch: (
    assetIds: string[],
    values: { field_id: string; value: string | number | boolean | null }[]
  ) =>
    apiFetch<BulkPatchAssetFieldsResponse>('/api/v1/assets/bulk/fields', {
      method: 'PATCH',
      body: JSON.stringify({ asset_ids: assetIds, values }),
    }),
}

export const projectFieldApi = {
  /** GET /api/v1/projects/:id/fields — fetch field values for a project. */
  get: (projectId: string) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`),

  /** PATCH /api/v1/projects/:id/fields (editor+) — update field values for a project. */
  patch: (
    projectId: string,
    values: { field_id: string; value: string | number | boolean | null }[]
  ) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),
}
