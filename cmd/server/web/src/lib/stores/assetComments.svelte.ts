import { assetApi, type ShareComment } from '$lib/api'

let comments = $state<ShareComment[]>([])
let loading = $state(false)

export const assetCommentsStore = {
  get loading() {
    return loading
  },
  get comments() {
    return comments
  },

  async load(assetId: string) {
    if (loading) return
    loading = true
    try {
      comments = await assetApi.listAssetComments(assetId)
    } catch {
      // 401 redirect handled by api client
    } finally {
      loading = false
    }
  },
}
