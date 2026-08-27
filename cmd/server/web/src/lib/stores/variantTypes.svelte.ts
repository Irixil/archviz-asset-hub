export interface VariantTypeDefinition {
  value: string
  label: string
  category: 'image' | 'video' | 'audio'
  paramsExample: Record<string, unknown>
}

export const variantTypes: VariantTypeDefinition[] = [
  {
    value: 'image_resize',
    label: 'Image Resize',
    category: 'image',
    paramsExample: {
      width: 1280,
      height: 720,
      fit: 'cover',
      format: 'jpeg',
      quality: 85,
    },
  },
  {
    value: 'image_convert',
    label: 'Image Convert',
    category: 'image',
    paramsExample: { format: 'webp', quality: 85 },
  },
  {
    value: 'image_crop',
    label: 'Image Crop',
    category: 'image',
    paramsExample: {
      x: 0,
      y: 0,
      width: 800,
      height: 600,
      format: 'jpeg',
      quality: 85,
    },
  },
  {
    value: 'image_watermark',
    label: 'Image Watermark',
    category: 'image',
    paramsExample: {
      watermark_asset_id: '',
      opacity: 0.5,
      format: 'jpeg',
      quality: 85,
    },
  },
  {
    value: 'image_bg_remove',
    label: 'Background Remove',
    category: 'image',
    paramsExample: {},
  },
  {
    value: 'image_with_prompt',
    label: 'Image With Prompt',
    category: 'image',
    paramsExample: {
      prompt: 'a photo of a cat on a white background',
      model: 'google/nano-banana-2:free',
    },
  },
  {
    value: 'image_smart_crop',
    label: 'Smart Crop',
    category: 'image',
    paramsExample: { width: 800, height: 800, format: 'jpeg', quality: 85 },
  },
  {
    value: 'video_capture_image',
    label: 'Video Capture Frame',
    category: 'video',
    paramsExample: { timestamp: 1.0 },
  },
  {
    value: 'video_transcode',
    label: 'Video Transcode',
    category: 'video',
    paramsExample: { format: 'mp4', resolution: '720p', strip_audio: false },
  },
  {
    value: 'video_watermark',
    label: 'Video Watermark',
    category: 'video',
    paramsExample: {
      watermark_asset_id: '',
      opacity: 0.5,
      format: 'mp4',
      resolution: '',
      strip_audio: false,
    },
  },
  {
    value: 'video_extract',
    label: 'Extract Audio',
    category: 'video',
    paramsExample: { format: 'aac', bitrate: '128k', mono: false },
  },
  {
    value: 'audio_transcode',
    label: 'Transcode Audio',
    category: 'audio',
    paramsExample: { format: 'mp3', bitrate: '192k', mono: false },
  },
  {
    value: 'audio_normalize',
    label: 'Normalize Audio',
    category: 'audio',
    paramsExample: { format: 'mp3', target_lufs: -14.0, mono: false },
  },
]

export const variantTypeMap = new Map(variantTypes.map((v) => [v.value, v]))
