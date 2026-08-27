import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type Workflow = definitions['api.WorkflowResponse']
export type WorkflowRun = definitions['api.WorkflowRunResponse']
export type WorkflowRunStep = definitions['api.WorkflowRunStepResponse']
export type WorkflowTemplate = definitions['api.WorkflowTemplateResponse']
export type WorkflowTriggerResponse = definitions['api.WorkflowTriggerResponse']
export type BulkManualRunResponse = definitions['api.BulkManualRunResponse']
export type WorkflowListRunsResponse =
  definitions['api.WorkflowListRunsResponse']
export type WorkflowTokenResponse = definitions['api.WorkflowTokenResponse']

// Graph sub-types are not generated (graph is serialized as a JSON string in the API).
// Node schema config_schema is a recursive JSON Schema structure not expressible in swagger.
// Both sets of types are kept hand-written.

export interface WorkflowGraphPosition {
  x: number
  y: number
}

export interface WorkflowGraphNode {
  id: string
  type: string
  config: Record<string, unknown>
  position: WorkflowGraphPosition
}

export interface WorkflowGraphEdge {
  id?: string
  from_node: string
  from_port: string
  to_node: string
  to_port: string
}

export interface WorkflowGraph {
  nodes: WorkflowGraphNode[]
  edges: WorkflowGraphEdge[]
}

export interface WorkflowNodePort {
  id: string
  label: string
}

export interface WorkflowNodeConfigSchema {
  type?: string
  title?: string
  format?: string
  placeholder?: string
  required?: string[]
  enum?: string[]
  items?: WorkflowNodeConfigSchema
  properties?: Record<string, WorkflowNodeConfigSchema>
  additionalProperties?: boolean
}

export interface WorkflowNodeSchema {
  type: string
  label: string
  category: string
  description: string
  inputs: WorkflowNodePort[]
  outputs: WorkflowNodePort[]
  config_schema: WorkflowNodeConfigSchema
}

export interface CreateWorkflowBody {
  name: string
  description: string
  graph: string
  notify_on_failure_email?: string
}

export interface UpdateWorkflowBody {
  name: string
  description: string
  graph: string
  notify_on_failure_email?: string
}

export interface WorkflowEvent {
  id: string
  type: string
  payload: {
    asset_id?: string
    workflow_id?: string
    run_id?: string
    node_id?: string
    status?: string
    error?: string
  }
}

export const workflowsApi = {
  list: () => apiFetch<Workflow[]>('/api/v1/workflows'),
  listManual: () =>
    apiFetch<Workflow[]>(
      '/api/v1/workflows?trigger_type=trigger.manual&enabled_only=true'
    ),
  get: (id: string) => apiFetch<Workflow>(`/api/v1/workflows/${id}`),
  create: (body: CreateWorkflowBody) =>
    apiFetch<Workflow>('/api/v1/workflows', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  update: (id: string, body: UpdateWorkflowBody) =>
    apiFetch<Workflow>(`/api/v1/workflows/${id}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  setEnabled: (id: string, enabled: boolean) =>
    apiFetch<void>(`/api/v1/workflows/${id}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }),
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/workflows/${id}`, {
      method: 'DELETE',
    }),
  triggerManual: (id: string) =>
    apiFetch<WorkflowTriggerResponse>(`/api/v1/workflows/${id}/runs`, {
      method: 'POST',
    }),
  triggerBulk: (id: string, assetIds: string[]) =>
    apiFetch<BulkManualRunResponse>(`/api/v1/workflows/${id}/runs/bulk`, {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds }),
    }),
  listAllRuns: (cursor?: string) =>
    apiFetch<WorkflowListRunsResponse>(
      `/api/v1/workflows/runs${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`
    ),
  listRuns: (id: string, cursor?: string) =>
    apiFetch<WorkflowRun[]>(
      `/api/v1/workflows/${id}/runs${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`
    ),
  getRun: (id: string, runId: string) =>
    apiFetch<WorkflowRun>(`/api/v1/workflows/${id}/runs/${runId}`),
  getNodeSchemas: () =>
    apiFetch<WorkflowNodeSchema[]>('/api/v1/workflows/node-schemas'),
  getTemplates: () =>
    apiFetch<WorkflowTemplate[]>('/api/v1/workflows/templates'),
  getWebhookToken: (id: string) =>
    apiFetch<WorkflowTokenResponse>(`/api/v1/workflows/${id}/webhook-token`),
  regenerateWebhook: (id: string) =>
    apiFetch<WorkflowTokenResponse>(
      `/api/v1/workflows/${id}/webhook-token/regenerate`,
      {
        method: 'POST',
      }
    ),
}
