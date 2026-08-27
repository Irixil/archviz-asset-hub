<script lang="ts">
  import type { VisualSimilarResult } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { ArrowLeft, Grid2X2, Images } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'

  interface Props {
    anchorAssetId: string
    results: VisualSimilarResult[]
    onBack: () => void
    onSelect: (result: VisualSimilarResult) => void
    onOpenGridView: (assetId: string) => void
  }

  let { anchorAssetId, results, onBack, onSelect, onOpenGridView }: Props =
    $props()

  const count = $derived(results.length)
  const countLabel = $derived(m.result_count({ count }))

  function openGridView() {
    onOpenGridView(anchorAssetId)
    onBack()
  }
</script>

<div
  class="flex h-full flex-col overflow-hidden"
  in:fly={{ x: 24, duration: 200, easing: cubicOut }}
>
  <!-- Header -->
  <div
    class="flex items-center gap-3 border-b border-[var(--border-subtle)] px-4 py-3"
  >
    <button
      type="button"
      class="flex items-center gap-1.5 text-sm text-[var(--text-muted)] transition-colors hover:text-[var(--text-primary)]"
      onclick={onBack}
    >
      <ArrowLeft class="h-4 w-4" />
      {m.back()}
    </button>
    <div class="flex-1">
      <h3 class="text-sm font-semibold text-[var(--text-primary)]">
        {m.results_title()}
      </h3>
      <p class="text-xs text-[var(--text-muted)]">{countLabel}</p>
    </div>
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
      onclick={openGridView}
    >
      <Grid2X2 class="h-3.5 w-3.5" />
      {m.similarity_open_grid_view()}
    </button>
  </div>

  <!-- Results list -->
  <div class="flex-1 overflow-y-auto">
    {#each results as result (result.asset_version_id)}
      <button
        type="button"
        class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-[var(--bg-hover)]"
        onclick={() => onSelect(result)}
      >
        <!-- Thumbnail -->
        <div
          class="h-12 w-12 shrink-0 overflow-hidden rounded-lg bg-[var(--bg-elevated)]"
        >
          {#if result.thumbnail_url}
            <img
              src={result.thumbnail_url}
              alt={result.original_filename}
              class="h-full w-full object-cover"
            />
          {:else}
            <div class="flex h-full w-full items-center justify-center">
              <Images class="h-5 w-5 text-[var(--text-muted)]" />
            </div>
          {/if}
        </div>

        <!-- Info -->
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-[var(--text-primary)]">
            {result.original_filename}
          </p>
          <p class="text-xs text-[var(--text-muted)]">
            {#if result.width && result.height}
              {result.width} × {result.height} ·
            {/if}
            {result.mime_type.split('/')[1]?.toUpperCase() ?? result.mime_type}
          </p>
        </div>
      </button>
    {/each}
  </div>
</div>
