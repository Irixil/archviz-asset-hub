import { assetApi, variantApi, type Asset } from '$lib/api'
import { thumbnailFileError } from './assetFiles'

const POLL_ATTEMPTS = 40
const POLL_INTERVAL_MS = 500

function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function setManualAssetThumbnail(
  assetId: string,
  file: File
): Promise<Asset> {
  const validationError = thumbnailFileError(file)
  if (validationError) throw new Error(validationError)

  const uploaded = await variantApi.uploadManual(assetId, file)
  await variantApi
    .patch(assetId, uploaded.id, { title: file.name })
    .catch(() => undefined)

  for (let attempt = 0; attempt < POLL_ATTEMPTS; attempt++) {
    const result = await variantApi.list(assetId)
    const ready = result.variants.find(
      (variant) => variant.id === uploaded.id && variant.thumbnail_url
    )
    if (ready) {
      await variantApi.setThumbnail(assetId, uploaded.id)
      return assetApi.get(assetId)
    }
    await wait(POLL_INTERVAL_MS)
  }

  throw new Error('缩略图处理超时，请稍后在素材详情中重试')
}
