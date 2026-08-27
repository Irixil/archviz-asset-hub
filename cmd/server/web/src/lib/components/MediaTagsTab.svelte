<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { assetFieldApi } from '$lib/api/custom_fields'
  import type { AssetFieldValue } from '$lib/api'
  import MetaTagsGroup from './MetaTagsGroup.svelte'
  import MetaTagRow from './MetaTagRow.svelte'

  interface Props {
    assetId: string
  }

  let { assetId }: Props = $props()

  let tags = $state<Map<string, AssetFieldValue> | null>(null)
  let empty = $state(false)
  let error = $state<string | null>(null)

  $effect(() => {
    let cancelled = false
    tags = null
    empty = false
    error = null

    assetFieldApi
      .get(assetId)
      .then((result) => {
        if (cancelled) return
        const mediaTagValues = result.fields.filter(
          (field) =>
            field.source === 'media_tags' || field.key.startsWith('_media_')
        )
        if (mediaTagValues.length === 0) {
          empty = true
          return
        }
        tags = new Map(mediaTagValues.map((field) => [field.key, field]))
      })
      .catch((err: unknown) => {
        if (cancelled) return
        error = err instanceof Error ? err.message : m.load_page_failed()
      })

    return () => {
      cancelled = true
    }
  })

  function val(key: string): string | number | boolean | null {
    return (tags?.get(key)?.value ?? null) as string | number | boolean | null
  }

  function fmtBitrate(bps: number | null): string | null {
    if (bps == null) return null
    if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mbps`
    return `${Math.round(bps / 1000)} kbps`
  }

  function fmtDuration(sec: number | null): string | null {
    if (sec == null) return null
    const h = Math.floor(sec / 3600)
    const min = Math.floor((sec % 3600) / 60)
    const s = Math.floor(sec % 60)
    return h > 0
      ? `${h}:${String(min).padStart(2, '0')}:${String(s).padStart(2, '0')}`
      : `${min}:${String(s).padStart(2, '0')}`
  }

  function fmtTrack(num: number | null, total: number | null): string | null {
    if (num == null) return null
    return total != null ? `${num} / ${total}` : `${num}`
  }

  function fmtFrameRate(rate: string | null): string | null {
    if (!rate) return null
    const [a, b] = rate.split('/').map(Number)
    if (!b) return null
    const fps = a / b
    return `${fps % 1 === 0 ? fps : fps.toFixed(3)} fps`
  }

  function fmtChannels(n: number | null, layout: string | null): string | null {
    if (n == null) return null
    if (layout) return layout.charAt(0).toUpperCase() + layout.slice(1)
    if (n === 1) return 'Mono'
    if (n === 2) return 'Stereo'
    return `${n} ch`
  }

  function yesNo(v: boolean | null): string | null {
    if (v == null) return null
    return v ? m.yes() : m.no()
  }
</script>

<div class="space-y-4 px-5 py-5">
  {#if error}
    <p class="text-sm text-rose-500">{error}</p>
  {:else if empty}
    <p class="text-sm text-[var(--text-muted)]">{m.media_tags_no_data()}</p>
  {:else if !tags}
    <p class="text-sm text-[var(--text-muted)]">{m.media_tags_loading()}</p>
  {:else}
    <MetaTagsGroup title={m.media_tags_group_track()}>
      <MetaTagRow
        label={m.media_tags_field_title()}
        value={val('_media_title')}
      />
      <MetaTagRow
        label={m.media_tags_field_artist()}
        value={val('_media_artist')}
      />
      <MetaTagRow
        label={m.media_tags_field_album()}
        value={val('_media_album')}
      />
      <MetaTagRow
        label={m.media_tags_field_album_artist()}
        value={val('_media_album_artist')}
      />
      <MetaTagRow
        label={m.media_tags_field_track()}
        value={fmtTrack(
          val('_media_track_number') as number | null,
          val('_media_track_total') as number | null
        )}
      />
      <MetaTagRow
        label={m.media_tags_field_disc()}
        value={fmtTrack(
          val('_media_disc_number') as number | null,
          val('_media_disc_total') as number | null
        )}
      />
      <MetaTagRow
        label={m.media_tags_field_year()}
        value={val('_media_year')}
      />
      <MetaTagRow
        label={m.media_tags_field_genre()}
        value={val('_media_genre')}
      />
      <MetaTagRow label={m.media_tags_field_bpm()} value={val('_media_bpm')} />
      <MetaTagRow
        label={m.media_tags_field_compilation()}
        value={yesNo(val('_media_compilation') as boolean | null)}
      />
    </MetaTagsGroup>

    <MetaTagsGroup title={m.media_tags_group_credits()}>
      <MetaTagRow
        label={m.media_tags_field_composer()}
        value={val('_media_composer')}
      />
      <MetaTagRow
        label={m.media_tags_field_lyricist()}
        value={val('_media_lyricist')}
      />
      <MetaTagRow
        label={m.media_tags_field_copyright()}
        value={val('_media_copyright')}
      />
      <MetaTagRow
        label={m.media_tags_field_encoder()}
        value={val('_media_encoder')}
      />
      <MetaTagRow
        label={m.media_tags_field_encoded_by()}
        value={val('_media_encoded_by')}
      />
      <MetaTagRow
        label={m.media_tags_field_language()}
        value={val('_media_language')}
      />
    </MetaTagsGroup>

    <MetaTagsGroup title={m.media_tags_group_technical()}>
      <MetaTagRow
        label={m.media_tags_field_container()}
        value={(val('_media_container') as string | null)?.toUpperCase() ??
          null}
      />
      <MetaTagRow
        label={m.media_tags_field_duration()}
        value={fmtDuration(val('_media_duration_sec') as number | null)}
      />
      <MetaTagRow
        label={m.media_tags_field_overall_bitrate()}
        value={fmtBitrate(val('_media_overall_bitrate') as number | null)}
      />
      <MetaTagRow
        label={m.media_tags_field_audio_codec()}
        value={(val('_media_audio_codec') as string | null)?.toUpperCase() ??
          null}
      />
      <MetaTagRow
        label={m.media_tags_field_audio_bitrate()}
        value={fmtBitrate(val('_media_audio_bitrate') as number | null)}
      />
      <MetaTagRow
        label={m.media_tags_field_sample_rate()}
        value={val('_media_sample_rate') != null
          ? `${val('_media_sample_rate')} Hz`
          : null}
      />
      <MetaTagRow
        label={m.media_tags_field_channels()}
        value={fmtChannels(
          val('_media_channels') as number | null,
          val('_media_channel_layout') as string | null
        )}
      />
      <MetaTagRow
        label={m.media_tags_field_bit_depth()}
        value={val('_media_bits_per_sample') != null
          ? `${val('_media_bits_per_sample')}-bit`
          : null}
      />
      <MetaTagRow
        label={m.media_tags_field_cover_art()}
        value={yesNo(val('_media_has_cover_art') as boolean | null)}
      />
    </MetaTagsGroup>

    {#if val('_media_video_codec')}
      <MetaTagsGroup title={m.media_tags_group_video()}>
        <MetaTagRow
          label={m.media_tags_field_video_codec()}
          value={(val('_media_video_codec') as string | null)?.toUpperCase() ??
            null}
        />
        <MetaTagRow
          label={m.media_tags_field_resolution()}
          value={val('_media_video_width') != null
            ? `${val('_media_video_width')} × ${val('_media_video_height')}`
            : null}
        />
        <MetaTagRow
          label={m.media_tags_field_frame_rate()}
          value={fmtFrameRate(val('_media_frame_rate') as string | null)}
        />
      </MetaTagsGroup>
    {/if}

    {#if val('_media_lyrics')}
      <MetaTagsGroup
        title={m.media_tags_group_lyrics()}
        collapsible
        initiallyCollapsed
      >
        <div class="col-span-2">
          <pre
            class="text-sm break-words whitespace-pre-wrap text-[var(--text-primary)]">{val(
              '_media_lyrics'
            )}</pre>
        </div>
      </MetaTagsGroup>
    {/if}

    {#if val('_media_comment')}
      <MetaTagsGroup title={m.media_tags_group_comment()}>
        <div class="col-span-2 text-sm text-[var(--text-primary)]">
          {val('_media_comment')}
        </div>
      </MetaTagsGroup>
    {/if}
  {/if}
</div>
