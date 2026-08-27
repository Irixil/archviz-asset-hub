import { apiFetch } from '$lib/api/client'
import type { definitions } from './types.gen'

type ArrayElement<MaybeArrayType> =
  MaybeArrayType extends readonly (infer ElementType)[]
    ? ElementType extends unknown[]
      ? ArrayElement<ElementType>
      : ElementType
    : MaybeArrayType

export type AIProviderStatus = definitions['api.AIProviderStatusResponse']
export type AIProviderModel = definitions['api.AIProviderModelResponse']
export type AIProviderList = definitions['api.AIProvidersListResponse']
export type AIProvider = ArrayElement<AIProviderList['providers']>

export interface AIProvidersResponse {
  providers: AIProviderStatus[]
}

export interface ProviderKeyStatus {
  key_set: boolean
  source: 'workspace' | 'env' | 'none'
}

export type ProviderId = 'openrouter' | 'imagerouter'

export const aiProvidersApi = {
  listAIProviders: async function (): Promise<AIProvidersResponse> {
    return apiFetch<AIProvidersResponse>('/api/v1/aiproviders')
  },

  getAIProviderKeyStatus: async function (
    provider: ProviderId
  ): Promise<ProviderKeyStatus> {
    return apiFetch<ProviderKeyStatus>(
      `/api/v1/workspace/settings/aiproviders/${provider}`
    )
  },

  setAIProviderKey: async function (
    provider: ProviderId,
    key: string
  ): Promise<void> {
    return apiFetch<void>(
      `/api/v1/workspace/settings/aiproviders/${provider}`,
      {
        method: 'PUT',
        body: JSON.stringify({ key }),
      }
    )
  },

  clearAIProviderKey: async function (provider: ProviderId): Promise<void> {
    return apiFetch<void>(
      `/api/v1/workspace/settings/aiproviders/${provider}`,
      {
        method: 'DELETE',
      }
    )
  },

  testAIProviderKey: async function (provider: ProviderId): Promise<void> {
    return apiFetch<void>(
      `/api/v1/workspace/settings/aiproviders/${provider}/test`,
      {
        method: 'POST',
      }
    )
  },
}
