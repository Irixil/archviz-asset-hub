<script lang="ts">
  import { textTrackApi, type TextTrack } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    track: TextTrack
    assetId: string
    onDeleted: (id: string) => void
  }

  let { track, assetId, onDeleted }: Props = $props()

  let expanded = $state(false)
  let deleting = $state(false)
  const previewLen = 220

  async function handleDelete() {
    if (!confirm(m.text_tracks_delete_confirm())) return
    deleting = true
    try {
      await textTrackApi.delete(assetId, track.id)
      onDeleted(track.id)
    } finally {
      deleting = false
    }
  }
</script>

<article
  class="space-y-3 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-4 shadow-sm"
>
  <div class="flex items-start justify-between gap-3">
    <div class="flex flex-wrap items-center gap-2">
      <span
        class="rounded-full bg-[var(--tag-bg)] px-2.5 py-1 text-xs font-medium text-[var(--tag-text)]"
      >
        {track.source === 'ocr'
          ? m.text_tracks_source_ocr()
          : m.text_tracks_source_manual()}
      </span>
      {#if track.lang}
        <span
          class="rounded-full border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--text-muted)]"
        >
          {track.lang}
        </span>
      {/if}
      {#if track.status !== 'ready'}
        <span
          class="rounded-full border border-amber-200 px-2.5 py-1 text-xs text-amber-700 dark:border-amber-900 dark:text-amber-300"
        >
          {track.status === 'failed'
            ? m.text_tracks_status_failed()
            : m.text_tracks_status_pending()}
        </span>
      {/if}
    </div>

    <div class="flex items-center gap-2">
      {#if track.has_file}
        <a
          href={track.download_url ??
            textTrackApi.downloadUrl(assetId, track.id)}
          class="text-xs font-medium text-[var(--accent)] hover:underline"
        >
          {m.text_tracks_download()}
        </a>
      {/if}
      <button
        type="button"
        class="text-xs font-medium text-[var(--accent-danger)] hover:underline disabled:opacity-50"
        disabled={deleting}
        onclick={handleDelete}
      >
        {m.delete_permanently()}
      </button>
    </div>
  </div>

  {#if track.status === 'failed' && track.error}
    <p class="text-sm text-[var(--accent-danger)]">{track.error}</p>
  {:else if track.content}
    <p
      class="text-sm leading-6 whitespace-pre-wrap text-[var(--text-secondary)]"
    >
      {expanded || track.content.length <= previewLen
        ? track.content
        : `${track.content.slice(0, previewLen)}…`}
    </p>
    {#if track.content.length > previewLen}
      <button
        type="button"
        class="text-xs font-medium text-[var(--text-muted)] hover:underline"
        onclick={() => (expanded = !expanded)}
      >
        {expanded ? m.text_tracks_show_less() : m.text_tracks_show_more()}
      </button>
    {/if}
  {/if}
</article>
