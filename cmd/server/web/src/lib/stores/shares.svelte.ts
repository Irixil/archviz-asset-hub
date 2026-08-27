import { shareApi, type CreateShareParams, type Share } from '$lib/api'
import { toastStore } from './toast.svelte'

let shares = $state<Share[]>([])
let loading = $state(false)

export const EXPIRY_OPTIONS: { label: string; value: number | null }[] = [
  { label: '7 days', value: 7 },
  { label: '14 days', value: 14 },
  { label: '30 days', value: 30 },
  { label: 'Never', value: null },
]

export const sharesStore = {
  get shares() {
    return shares
  },
  get loading() {
    return loading
  },

  /** Return shares for a specific target (type + id). */
  forTarget(targetType: string, targetId: string): Share[] {
    return shares.filter(
      (s) =>
        s.target_type === targetType &&
        s.target_id === targetId &&
        !s.revoked_at
    )
  },

  async load() {
    loading = true
    try {
      shares = await shareApi.list()
    } catch {
      toastStore.show('Could not load shares', 'error')
    } finally {
      loading = false
    }
  },

  async create(params: CreateShareParams): Promise<Share | null> {
    try {
      const share = await shareApi.create(params)
      shares = [share, ...shares]
      return share
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Could not create share'
      toastStore.show(msg, 'error')
      return null
    }
  },

  async revoke(id: string) {
    try {
      await shareApi.revoke(id)
      shares = shares.map((s) =>
        s.id === id ? { ...s, revoked_at: new Date().toISOString() } : s
      )
      toastStore.show('Share revoked')
    } catch {
      toastStore.show('Could not revoke share', 'error')
    }
  },
}
