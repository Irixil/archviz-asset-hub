type DraggableAsset = {
  id: string
  original_filename: string
  mime_type: string
  storage_key?: string
}

const MODEL_EXTENSIONS = new Set([
  '3dm',
  '3ds',
  'abc',
  'blend',
  'c4d',
  'cgeo',
  'dae',
  'fbx',
  'glb',
  'gltf',
  'max',
  'obj',
  'rvt',
  'skp',
  'vrmesh',
])
const PSD_EXTENSIONS = new Set(['psb', 'psd'])
const CAD_EXTENSIONS = new Set(['dgn', 'dwg', 'dxf'])
const PACKAGE_EXTENSIONS = new Set(['7z', 'rar', 'zip'])
const IMAGE_EXTENSIONS = new Set([
  'avif',
  'bmp',
  'gif',
  'heic',
  'heif',
  'jpeg',
  'jpg',
  'png',
  'svg',
  'tif',
  'tiff',
  'webp',
])
const THUMBNAIL_EXTENSIONS = new Set(['jpeg', 'jpg', 'png', 'webp'])
const MAX_THUMBNAIL_BYTES = 10 * 1024 * 1024

export type PlannedAssetUpload = {
  file: File
  thumbnail?: File
}

export function fileExtension(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return dot > -1 ? filename.slice(dot + 1).toLowerCase() : ''
}

export function assetPrimaryCategory(
  filename: string,
  mimeType: string
): string | undefined {
  const ext = fileExtension(filename)
  if (MODEL_EXTENSIONS.has(ext)) return '3D模型'
  if (PSD_EXTENSIONS.has(ext)) return 'PSD素材'
  if (ext === 'hdr' || ext === 'exr') return 'HDRI环境'
  if (CAD_EXTENSIONS.has(ext)) return 'CAD图纸'
  if (ext === 'ies') return 'IES灯光'
  if (PACKAGE_EXTENSIONS.has(ext)) return '素材包'
  if (IMAGE_EXTENSIONS.has(ext)) return '图片素材'
  if (mimeType.startsWith('image/')) return '图片素材'
}

export function assetTypeLabel(filename: string, mimeType: string): string {
  const primaryCategory = assetPrimaryCategory(filename, mimeType)
  if (primaryCategory) return primaryCategory
  if (mimeType.startsWith('video/')) return '视频'
  if (mimeType.startsWith('audio/')) return '音频'
  if (mimeType === 'application/pdf') return 'PDF 文档'
  return '文档'
}

export function assetFormatLabel(filename: string, mimeType: string): string {
  const ext = fileExtension(filename)
  if (ext) return ext.toUpperCase()
  return mimeType.split('/').pop()?.toUpperCase() ?? '文件'
}

export function canGenerateThumbnail(
  filename: string,
  mimeType: string
): boolean {
  const ext = fileExtension(filename)
  if (MODEL_EXTENSIONS.has(ext) || PSD_EXTENSIONS.has(ext)) return false
  return (
    mimeType.startsWith('image/') ||
    mimeType.startsWith('video/') ||
    mimeType.startsWith('audio/') ||
    mimeType === 'application/pdf' ||
    mimeType.startsWith('text/')
  )
}

export function thumbnailFileError(file: File): string | null {
  if (!THUMBNAIL_EXTENSIONS.has(fileExtension(file.name))) {
    return '请选择 JPG、PNG 或 WebP 图片'
  }
  if (file.size > MAX_THUMBNAIL_BYTES) {
    return '缩略图不能超过 10 MB'
  }
  return null
}

function fileStem(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return (dot > 0 ? filename.slice(0, dot) : filename).trim().toLowerCase()
}

/**
 * Treat an exact-name image uploaded beside a MAX file as that model's cover.
 * The image becomes a manual variant instead of a second top-level asset.
 */
export function planAssetUploads(files: readonly File[]): PlannedAssetUpload[] {
  const previews = new Map<string, File[]>()
  for (const file of files) {
    if (thumbnailFileError(file)) continue
    const stem = fileStem(file.name)
    previews.set(stem, [...(previews.get(stem) ?? []), file])
  }

  const paired = new Map<File, File>()
  const consumedPreviews = new Set<File>()
  for (const file of files) {
    if (fileExtension(file.name) !== 'max') continue
    const preview = previews
      .get(fileStem(file.name))
      ?.find((candidate) => !consumedPreviews.has(candidate))
    if (!preview) continue
    paired.set(file, preview)
    consumedPreviews.add(preview)
  }

  return files
    .filter((file) => !consumedPreviews.has(file))
    .map((file) => ({ file, thumbnail: paired.get(file) }))
}

export function setAssetDragData(
  event: DragEvent,
  asset: DraggableAsset,
  fileUrl: string
) {
  if (window.archvizDesktop?.isAvailable && asset.storage_key) {
    event.preventDefault()
    window.archvizDesktop.startDrag({
      storageKey: asset.storage_key,
    })
    return 'native'
  }

  if (!event.dataTransfer) return 'none'
  const absoluteUrl = new URL(fileUrl, window.location.href).href
  event.dataTransfer.setData('text/plain', asset.id)
  event.dataTransfer.setData(
    'application/json',
    JSON.stringify({ assetId: asset.id })
  )
  event.dataTransfer.setData('text/uri-list', absoluteUrl)
  event.dataTransfer.setData(
    'DownloadURL',
    `${asset.mime_type}:${asset.original_filename}:${absoluteUrl}`
  )
  return 'browser'
}
