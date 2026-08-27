import { apiFetch } from './client'
import type { definitions } from './types.gen'

export type TelemetryStatus = definitions['api.TelemetryStatusResponse']

export const telemetryApi = {
  getStatus: () => apiFetch<TelemetryStatus>('/api/admin/telemetry'),
}
