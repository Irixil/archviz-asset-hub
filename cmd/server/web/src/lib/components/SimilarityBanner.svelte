<script lang="ts">
  import { assetApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { X } from '@lucide/svelte'

  type Props = {
    anchorFilename: string
    anchorAssetId: string
    resultCount: number
    onClear: () => void
    onOpenAnchor: () => void
  }

  let {
    anchorFilename,
    anchorAssetId,
    resultCount,
    onClear,
    onOpenAnchor,
  }: Props = $props()
</script>

<div
  class="flex min-h-10 items-center gap-3 border-y border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2 sm:px-6"
>
  <img
    src={assetApi.thumbUrl(anchorAssetId)}
    alt={anchorFilename}
    class="h-7 w-7 shrink-0 rounded object-cover ring-1 ring-[var(--border-default)]"
    loading="lazy"
  />
  <div class="min-w-0 flex-1 text-sm">
    <span class="font-medium text-[var(--text-secondary)]"
      >{m.similarity_banner_label()}</span
    >
    <button
      type="button"
      class="ml-1 font-semibold text-[var(--text-primary)] underline-offset-2 hover:underline"
      onclick={onOpenAnchor}>{anchorFilename}</button
    >
    <span class="ml-2 text-xs text-[var(--text-muted)]"
      >{m.similarity_result_count({ count: resultCount })}</span
    >
  </div>
  <button
    type="button"
    class="rounded p-1 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
    aria-label={m.cancel()}
    onclick={onClear}
  >
    <X class="h-4 w-4" />
  </button>
</div>
