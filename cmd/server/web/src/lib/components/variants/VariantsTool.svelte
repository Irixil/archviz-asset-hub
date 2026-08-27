<script lang="ts">
  import { type Asset } from '$lib/api'
  import VariantCreateImageResize from './VariantCreateImageResize.svelte'
  import VariantCreateImageWatermark from './VariantCreateImageWatermark.svelte'
  import VariantCreateImageConvert from './VariantCreateImageConvert.svelte'
  import VariantCreateImageCrop from './VariantCreateImageCrop.svelte'
  import VariantCreateImageRemoveBackground from './VariantCreateImageRemoveBackground.svelte'
  import VariantCreateImageWithPrompt from './VariantCreateImageWithPrompt.svelte'
  import VariantCreateImageSmartCrop from './VariantCreateImageSmartCrop.svelte'
  import VariantCreateVideoThumbnail from './VariantCreateVideoThumbnail.svelte'
  import VariantCreateVideoTranscode from './VariantCreateVideoTranscode.svelte'
  import VariantCreateVideoWatermark from './VariantCreateVideoWatermark.svelte'
  import VariantCreateVideoExtract from './VariantCreateVideoExtract.svelte'
  import VariantCreateAudioTranscode from './VariantCreateAudioTranscode.svelte'
  import VariantCreateAudioNormalize from './VariantCreateAudioNormalize.svelte'
  import VariantCreateCustomFFmpeg from './VariantCreateCustomFFmpeg.svelte'

  export type VariantTab =
    | 'all'
    | 'image_resize'
    | 'image_watermark'
    | 'image_convert'
    | 'image_smart_crop'
    | 'image_crop'
    | 'image_bg_remove'
    | 'image_with_prompt'
    | 'video_transcode'
    | 'video_watermark'
    | 'video_capture_image'
    | 'video_extract'
    | 'audio_transcode'
    | 'audio_normalize'
    | 'custom_ffmpeg'
    | 'trigger_workflow'

  interface Props {
    asset: Asset
    tool: VariantTab
    creating?: boolean
    handleCreate: (type: string, params: object) => void
    onDone?: () => void
    onDraftStarted?: (nonce: string, meta?: Record<string, unknown>) => void
    sessionActive?: boolean
    initialParams?: Record<string, unknown> | null
  }
  let {
    asset,
    tool,
    creating = false,
    handleCreate,
    onDone,
    onDraftStarted,
    sessionActive,
    initialParams = null,
  }: Props = $props()
</script>

{#if tool === 'image_resize'}
  <VariantCreateImageResize {asset} {creating} {handleCreate} {initialParams} />
{:else if tool === 'image_watermark'}
  <VariantCreateImageWatermark
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
    {initialParams}
  />
{:else if tool === 'image_convert'}
  <VariantCreateImageConvert
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'image_crop'}
  <VariantCreateImageCrop {asset} {creating} {handleCreate} {initialParams} />
{:else if tool === 'image_bg_remove'}
  <VariantCreateImageRemoveBackground
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
    {initialParams}
  />
{:else if tool === 'image_with_prompt'}
  <VariantCreateImageWithPrompt
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
    {initialParams}
  />
{:else if tool === 'image_smart_crop'}
  <VariantCreateImageSmartCrop
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'video_transcode'}
  <VariantCreateVideoTranscode
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'video_watermark'}
  <VariantCreateVideoWatermark
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'video_capture_image'}
  <VariantCreateVideoThumbnail
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'video_extract'}
  <VariantCreateVideoExtract
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'audio_transcode'}
  <VariantCreateAudioTranscode
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'audio_normalize'}
  <VariantCreateAudioNormalize
    {asset}
    {creating}
    {handleCreate}
    {initialParams}
  />
{:else if tool === 'custom_ffmpeg'}
  <VariantCreateCustomFFmpeg
    {asset}
    {creating}
    {handleCreate}
    {onDraftStarted}
    {sessionActive}
    {initialParams}
  />
{/if}
