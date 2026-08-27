<script lang="ts">
  import { assetApi, formatBytes, type Asset } from '$lib/api'
  import { stackApi } from '$lib/api/client'
  import {
    ChevronDown,
    Download,
    GripVertical,
    ImagePlus,
    LoaderCircle,
    RotateCcw,
  } from '@lucide/svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import { DOWNLOAD_BUTTON_COLORS } from '$lib/stores/assetView'
  import { m } from '$lib/paraglide/messages'
  import {
    assetFormatLabel,
    assetTypeLabel,
    setAssetDragData,
  } from '$lib/utils/assetFiles'
  import { setManualAssetThumbnail } from '$lib/utils/assetThumbnails'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'

  type Props = {
    asset: Asset
    category: string
    onThumbnailUpdated?: (updated: Asset) => void
  }

  let { asset, category, onThumbnailUpdated }: Props = $props()

  let retriedClassificationStatus = $state<'classified'>()
  let retriedClassificationCategory = $state<string>()
  const classificationStatus = $derived(
    retriedClassificationStatus ?? asset.classification_status
  )
  const classificationCategory = $derived(
    retriedClassificationCategory ?? asset.classification_category
  )
  let classifying = $state(false)
  let classificationError = $state('')
  let thumbnailInput: HTMLInputElement | undefined = $state()
  let thumbnailBusy = $state(false)

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('zh-CN', {
      month: 'long',
      day: 'numeric',
      year: 'numeric',
    })
  }

  let details: HTMLDetailsElement | undefined = $state()

  function closeDropdown() {
    details?.removeAttribute('open')
  }

  async function downloadZip(variantMode: 'shared' | 'all') {
    closeDropdown()
    await stackApi.exportZip([asset.id], asset.original_filename, variantMode)
  }

  async function retryClassification() {
    classifying = true
    classificationError = ''
    try {
      const result = await assetApi.classify(asset.id)
      retriedClassificationStatus = result.classification_status
      retriedClassificationCategory = result.classification_category
    } catch (error) {
      classificationError =
        error instanceof Error ? error.message : '重新分类失败'
    } finally {
      classifying = false
    }
  }

  async function changeThumbnail(event: Event) {
    const input = event.currentTarget as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return

    thumbnailBusy = true
    try {
      const updated = await setManualAssetThumbnail(asset.id, file)
      assetsStore.patchAsset(asset.id, updated)
      assetsStore.reloadAssetResources(asset.id)
      onThumbnailUpdated?.(updated)
      toastStore.show('缩略图已更新', 'success')
    } catch (error) {
      toastStore.show(
        error instanceof Error ? error.message : '缩略图更新失败，请重试',
        'error'
      )
    } finally {
      thumbnailBusy = false
    }
  }
</script>

<div class="flex items-start justify-between gap-3">
  <div class="min-w-0">
    <!-- Pills row -->
    <div class="mb-1.5 flex flex-wrap items-center gap-1.5">
      <Badge
        variant={category as
          | 'image'
          | 'video'
          | 'audio'
          | 'document'
          | 'neutral'}
        size="md"
      >
        {assetTypeLabel(asset.original_filename, asset.mime_type)} ·
        {assetFormatLabel(asset.original_filename, asset.mime_type)}
      </Badge>
      <span
        class="text-md rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
      >
        {formatBytes(asset.size)}
      </span>
      {#if asset.width != null && asset.height != null}
        <span
          class="text-md rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
        >
          {asset.width} × {asset.height}
        </span>
      {/if}
      {#if classificationStatus === 'needs_review'}
        <button
          type="button"
          class="inline-flex items-center gap-1 rounded bg-amber-100 px-1.5 py-0.5 text-sm font-medium text-amber-800 transition-colors hover:bg-amber-200 disabled:cursor-wait disabled:opacity-60"
          disabled={classifying}
          onclick={retryClassification}
          title={`按文件格式归入${classificationCategory ?? '对应分类'}`}
        >
          {#if classifying}
            <LoaderCircle class="h-3 w-3 animate-spin" />
            正在分类
          {:else}
            <RotateCcw class="h-3 w-3" />
            重新分类
          {/if}
        </button>
      {:else if classificationStatus === 'classified' && classificationCategory}
        <span
          class="rounded bg-emerald-50 px-1.5 py-0.5 text-sm font-medium text-emerald-700"
          >已归入 {classificationCategory}</span
        >
      {/if}
      {#if classificationError}
        <span class="text-sm text-red-600">{classificationError}</span>
      {/if}
    </div>
    <!-- Author + date -->
    <p class="text-md mt-0.5 text-[#8ba4c4] dark:text-[#5f7590]">
      {formatDate(asset.created_at)}
    </p>
    {#if authStore.role !== 'viewer'}
      <input
        bind:this={thumbnailInput}
        type="file"
        accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"
        class="hidden"
        onchange={changeThumbnail}
      />
      <button
        type="button"
        class="mt-2 inline-flex min-h-10 items-center gap-2 rounded-md border border-[var(--border-default)] bg-[var(--bg-surface)] px-3 text-sm font-semibold text-[var(--text-secondary)] transition-colors hover:border-[var(--accent)] hover:bg-[var(--accent-soft)] hover:text-[var(--text-primary)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--focus-ring)] disabled:cursor-wait disabled:opacity-60"
        disabled={thumbnailBusy}
        onclick={() => thumbnailInput?.click()}
      >
        {#if thumbnailBusy}
          <LoaderCircle class="h-4 w-4 animate-spin" />
          正在处理
        {:else}
          <ImagePlus class="h-4 w-4" />
          {asset.thumbnail_key ? '更换缩略图' : '添加缩略图'}
        {/if}
      </button>
    {/if}
  </div>

  <!-- Download dropdown -->
  <details bind:this={details} class="group relative shrink-0">
    <summary
      draggable="true"
      class="flex cursor-grab list-none items-center gap-1.5 rounded-xl px-3 py-2.5 text-white transition-colors active:cursor-grabbing {DOWNLOAD_BUTTON_COLORS[
        category
      ] ?? 'bg-indigo-600 hover:bg-indigo-700'}"
      aria-label="取用素材，可拖至 3ds Max 或 Photoshop"
      title="拖至 3ds Max、Photoshop 或文件夹，也可点击下载"
      ondragstart={(e) =>
        setAssetDragData(e, asset, assetApi.fileUrl(asset.id))}
    >
      <GripVertical class="h-4 w-4" />
      <span class="text-sm font-medium">取用素材</span>
      <ChevronDown class="h-3.5 w-3.5" />
    </summary>

    <div
      class="absolute right-0 top-full z-20 mt-1 w-52 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900"
    >
      <a
        href={assetApi.fileUrl(asset.id)}
        download={asset.original_filename}
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={closeDropdown}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.download_original()}
      </a>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={() => downloadZip('shared')}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.with_shared_variants()}
      </button>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={() => downloadZip('all')}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.with_all_variants()}
      </button>
    </div>
  </details>
</div>
