import { isAudio, isImage, isVideo } from '$lib/utils/mime'
import {
  AudioLines,
  Camera,
  Crop,
  EraserIcon,
  Film,
  Maximize2,
  Music,
  Scissors,
  Shapes,
  Sparkles,
  Stamp,
  Terminal,
  Video,
  Workflow,
} from '@lucide/svelte'
import type { Component } from 'svelte'
import type { VariantTab } from './VariantsTool.svelte'

export interface VariantToolDef {
  key: VariantTab
  label: string
  sublabel: string
  icon: Component
  showFor: (mimeType: string) => boolean
}

export const ALL_VARIANT_TOOLS: VariantToolDef[] = [
  {
    key: 'image_resize',
    label: 'variant_tool_resize',
    sublabel: 'variant_tool_resize_sub',
    icon: Maximize2,
    showFor: isImage,
  },
  {
    key: 'image_crop',
    label: 'variant_tool_crop',
    sublabel: 'variant_tool_crop_sub',
    icon: Crop,
    showFor: isImage,
  },
  {
    key: 'image_smart_crop',
    label: 'variant_tool_smart_crop',
    sublabel: 'variant_tool_smart_crop_sub',
    icon: Scissors,
    showFor: isImage,
  },
  {
    key: 'image_convert',
    label: 'variant_tool_convert',
    sublabel: 'variant_tool_convert_sub',
    icon: Shapes,
    showFor: isImage,
  },
  {
    key: 'image_with_prompt',
    label: 'variant_tool_ai_transform',
    sublabel: 'variant_tool_ai_transform_sub',
    icon: Sparkles,
    showFor: isImage,
  },
  {
    key: 'image_watermark',
    label: 'variant_tool_watermark',
    sublabel: 'variant_tool_watermark_sub',
    icon: Stamp,
    showFor: isImage,
  },
  {
    key: 'image_bg_remove',
    label: 'variant_tool_bg_remove',
    sublabel: 'variant_tool_bg_remove_sub',
    icon: EraserIcon,
    showFor: isImage,
  },
  {
    key: 'video_transcode',
    label: 'variant_tool_video_transcode',
    sublabel: 'variant_tool_video_transcode_sub',
    icon: Video,
    showFor: isVideo,
  },
  {
    key: 'video_watermark',
    label: 'variant_tool_video_watermark',
    sublabel: 'variant_tool_video_watermark_sub',
    icon: Film,
    showFor: isVideo,
  },
  {
    key: 'video_capture_image',
    label: 'variant_tool_video_capture',
    sublabel: 'variant_tool_video_capture_sub',
    icon: Camera,
    showFor: isVideo,
  },
  {
    key: 'video_extract',
    label: 'variant_tool_audio_extract',
    sublabel: 'variant_tool_audio_extract_sub',
    icon: Music,
    showFor: isVideo,
  },
  {
    key: 'audio_transcode',
    label: 'variant_tool_audio_transcode',
    sublabel: 'variant_tool_audio_transcode_sub',
    icon: Music,
    showFor: isAudio,
  },
  {
    key: 'audio_normalize',
    label: 'variant_tool_audio_normalize',
    sublabel: 'variant_tool_audio_normalize_sub',
    icon: AudioLines,
    showFor: isAudio,
  },
  {
    key: 'custom_ffmpeg',
    label: 'variant_tool_custom_ffmpeg',
    sublabel: 'variant_tool_custom_ffmpeg_sub',
    icon: Terminal,
    showFor: () => true,
  },
  {
    key: 'trigger_workflow',
    label: 'variant_tool_trigger_workflow',
    sublabel: 'variant_tool_trigger_workflow_sub',
    icon: Workflow,
    showFor: () => true,
  },
]

/**
 * Variant types with user-authored settings worth surfacing as param history.
 * image_bg_remove and video_capture_image are excluded — their params carry no
 * creative choice (model selection is a workspace default, not a per-use decision).
 */
export const PARAM_HISTORY_TOOLS = new Set<VariantTab>([
  'image_with_prompt',
  'image_watermark',
  'video_watermark',
  'image_resize',
  'image_convert',
  'image_smart_crop',
  'image_crop',
  'video_transcode',
  'audio_transcode',
  'audio_normalize',
  'custom_ffmpeg',
])
