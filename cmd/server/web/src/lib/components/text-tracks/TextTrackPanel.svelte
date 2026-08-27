<script lang="ts">
  import { textTrackApi, type Asset, type TextTrack } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import TextTrackCard from './TextTrackCard.svelte'
  import TextTrackCreate from './TextTrackCreate.svelte'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let tracks = $state<TextTrack[]>([])
  let loading = $state(true)
  let showCreate = $state(false)
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null)

  async function load() {
    loading = true
    try {
      tracks = await textTrackApi.list(asset.id)
    } finally {
      loading = false
    }
  }

  function syncPolling() {
    const hasPending = tracks.some(
      (track) => track.status === 'pending' || track.status === 'processing'
    )
    if (hasPending && !pollTimer) {
      pollTimer = setInterval(async () => {
        await load()
        if (
          !tracks.some(
            (track) =>
              track.status === 'pending' || track.status === 'processing'
          )
        ) {
          clearInterval(pollTimer!)
          pollTimer = null
        }
      }, 3000)
    }
    if (!hasPending && pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  $effect(() => {
    asset.id
    showCreate = false
    void load()
    return () => {
      if (pollTimer) {
        clearInterval(pollTimer)
        pollTimer = null
      }
    }
  })

  $effect(() => {
    syncPolling()
  })
</script>

<section class="space-y-4 px-5 py-5">
  <div class="flex items-center justify-between gap-3">
    <div>
      <h3
        class="text-sm font-semibold tracking-[0.08em] text-[var(--text-primary)] uppercase"
      >
        {m.text_tracks_panel_title()}
      </h3>
      <p class="mt-1 text-sm text-[var(--text-muted)]">
        {m.text_tracks_empty_hint()}
      </p>
    </div>
    <button
      type="button"
      class="rounded-lg border border-[var(--border)] px-3 py-2 text-sm font-medium whitespace-nowrap text-[var(--text-primary)] transition hover:bg-[var(--bg-hover)]"
      onclick={() => (showCreate = !showCreate)}
    >
      {showCreate ? m.cancel() : m.text_tracks_add()}
    </button>
  </div>

  {#if showCreate}
    <TextTrackCreate
      {asset}
      onCreated={async () => {
        showCreate = false
        await load()
      }}
    />
  {/if}

  {#if loading}
    <p class="text-sm text-[var(--text-muted)]">{m.loading()}</p>
  {:else if tracks.length === 0}
    <p
      class="rounded-lg border border-dashed border-[var(--border)] px-4 py-6 text-sm text-[var(--text-muted)]"
    >
      {m.text_tracks_empty()}
    </p>
  {:else}
    <div class="space-y-3">
      {#each tracks as track (track.id)}
        <TextTrackCard
          {track}
          assetId={asset.id}
          onDeleted={(id) => {
            tracks = tracks.filter((entry) => entry.id !== id)
          }}
        />
      {/each}
    </div>
  {/if}
</section>
