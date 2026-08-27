import type { Variant } from '$lib/api'

export const isImage = (mime: string): boolean => mime.startsWith('image/')
export const isVideo = (mime: string): boolean => mime.startsWith('video/')
export const isAudio = (mime: string): boolean => mime.startsWith('audio/')
export const isPdf = (mime: string): boolean => mime === 'application/pdf'
export const isDocument = (mime: string): boolean => {
  switch (mime) {
    case 'application/vnd.oasis.opendocument.presentation':
    case 'application/vnd.ms-powerpoint':
    case 'application/vnd.openxmlformats-officedocument.presentationml.presentation':
    case 'application/vnd.oasis.opendocument.text':
    case 'application/msword':
    case 'application/vnd.openxmlformats-officedocument.wordprocessingml.document':
    case 'application/rtf':
    case 'text/html':
    case 'application/vnd.oasis.opendocument.spreadsheet':
    case 'application/vnd.ms-excel':
    case 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet':
    case 'text/csv':
      return true
    default:
      return false
  }
}

export type MimeCategory = 'image' | 'video' | 'audio' | 'document'

export function mimeCategory(mimeType: string): MimeCategory {
  if (isImage(mimeType)) return 'image'
  if (isVideo(mimeType)) return 'video'
  if (isAudio(mimeType)) return 'audio'
  return 'document'
}

export const MIME_EXT: Record<string, string> = {
  'image/jpeg': '.jpg',
  'image/png': '.png',
  'image/webp': '.webp',
  'image/gif': '.gif',
  'image/avif': '.avif',
  'video/mp4': '.mp4',
  'video/webm': '.webm',
  'audio/mpeg': '.mp3',
  'audio/mp4': '.m4a',
  'audio/wav': '.wav',
  'application/pdf': '.pdf',
}

export function extFromMime(mime: string): string {
  return MIME_EXT[mime.split(';')[0].trim()] ?? ''
}

export function deriveVariantMime(
  variant: Variant,
  fallbackMime: string
): string {
  const key = variant.storage_key ?? ''
  if (key.endsWith('.mp4') || key.endsWith('.m4v')) return 'video/mp4'
  if (key.endsWith('.webm')) return 'video/webm'
  if (key.endsWith('.mov')) return 'video/quicktime'
  if (key.endsWith('.mp3')) return 'audio/mpeg'
  if (key.endsWith('.aac')) return 'audio/aac'
  if (key.endsWith('.ogg')) return 'audio/ogg'
  if (key.endsWith('.wav')) return 'audio/wav'
  if (key.endsWith('.flac')) return 'audio/flac'
  if (key.endsWith('.pdf')) return 'application/pdf'
  if (key.endsWith('.png')) return 'image/png'
  if (key.endsWith('.jpg') || key.endsWith('.jpeg')) return 'image/jpeg'
  if (key.endsWith('.webp')) return 'image/webp'
  if (key.endsWith('.gif')) return 'image/gif'
  if (key.endsWith('.avif')) return 'image/avif'
  const t = variant.type ?? ''
  if (t.startsWith('video_'))
    return fallbackMime.startsWith('video/') ? fallbackMime : 'video/mp4'
  if (t.startsWith('audio_'))
    return fallbackMime.startsWith('audio/') ? fallbackMime : 'audio/mpeg'
  if (t === 'video_capture_image') return 'image/jpeg'
  return fallbackMime || 'application/octet-stream'
}
