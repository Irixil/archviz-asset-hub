<script lang="ts">
  import { onMount } from 'svelte'
  import { tagApi, type Tag } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import {
    detailFacetsForGroup,
    missingTaxonomyEntries,
    normalizeTaxonomyTagName,
    taxonomyGroupById,
  } from '$lib/data/architectureTaxonomy'
  import { Check, ChevronDown, SlidersHorizontal, X } from '@lucide/svelte'

  interface Props {
    activeTags: string[]
    onchange: (tags: string[]) => void
  }

  let { activeTags, onchange }: Props = $props()
  let counts = $state<Record<string, number>>({})
  let resolvedNames = $state<Record<string, string>>({})
  let expanded = $state(false)

  const activeGroup = $derived(
    taxonomyGroupById(navigationStore.activeArchiveGroupId)
  )
  const detailState = $derived(
    activeGroup ? detailFacetsForGroup(activeGroup, activeTags) : null
  )
  const quickFacet = $derived(detailState?.facets[0] ?? null)
  const quickEntries = $derived(quickFacet?.entries.slice(0, 4) ?? [])

  function updateTags(tags: Tag[]) {
    counts = Object.fromEntries(
      tags.map((tag) => [normalizeTaxonomyTagName(tag.name), tag.asset_count])
    )
    resolvedNames = Object.fromEntries(
      tags.map((tag) => [normalizeTaxonomyTagName(tag.name), tag.name])
    )
  }

  async function loadAndCreateTaxonomy() {
    try {
      let tags = await tagApi.list({ system: true })
      const canCreate = authStore.role !== 'viewer' && authStore.canCreateTag
      if (canCreate) {
        const missing = missingTaxonomyEntries(
          tags.map((tag) => tag.name),
          activeGroup ? [activeGroup.id] : []
        )
        if (missing.length > 0) {
          await Promise.allSettled(
            missing.map((tag) =>
              tagApi.create(tag.name, tag.color, tag.groupName)
            )
          )
          tags = await tagApi.list({ system: true })
        }
      }
      updateTags(tags)
    } catch {
      counts = {}
      resolvedNames = {}
    }
  }

  function toggleTag(name: string, facetEntries: string[]) {
    const normalizedName = normalizeTaxonomyTagName(name)
    const facetNames = new Set(facetEntries.map(normalizeTaxonomyTagName))
    const remaining = activeTags.filter(
      (tag) => !facetNames.has(normalizeTaxonomyTagName(tag))
    )
    const selected = activeTags.some(
      (tag) => normalizeTaxonomyTagName(tag) === normalizedName
    )
    onchange(
      selected
        ? remaining
        : [...remaining, resolvedNames[normalizedName] ?? name]
    )
  }

  function removeTag(name: string) {
    const normalizedName = normalizeTaxonomyTagName(name)
    onchange(
      activeTags.filter(
        (tag) => normalizeTaxonomyTagName(tag) !== normalizedName
      )
    )
  }

  onMount(loadAndCreateTaxonomy)
</script>

{#if activeGroup}
  <section
    aria-label={`${activeGroup.label}分类筛选`}
    class="taxonomy-shell border-t border-[var(--border-subtle)] bg-[var(--bg-surface)]"
  >
    <div class="flex min-h-12 flex-wrap items-center gap-2 px-3 py-2 sm:px-4">
      <div class="mr-1 flex min-w-0 items-center gap-2">
        <div class="min-w-0">
          <h2 class="truncate text-sm font-semibold text-[var(--text-primary)]">
            {activeGroup.label}{#if detailState?.selectedPrimary}<span
                class="font-normal text-[var(--text-muted)]"
              >
                /
              </span><span class="text-[var(--accent-text)]"
                >{detailState.selectedPrimary}</span
              >{/if}
          </h2>
        </div>
      </div>

      {#if activeTags.length > 0}
        <div
          class="flex min-w-0 flex-1 flex-wrap items-center gap-1"
          aria-label="已选筛选条件"
        >
          {#each activeTags as tag (tag)}
            <button
              type="button"
              class="selected-chip"
              onclick={() => removeTag(tag)}
              aria-label={`移除筛选：${tag}`}
            >
              <Check class="h-3 w-3" />
              <span class="max-w-24 truncate">{tag}</span>
              <X class="h-3 w-3 opacity-75" />
            </button>
          {/each}
        </div>
      {:else if quickFacet}
        <div class="hidden min-w-0 flex-1 items-center gap-1 lg:flex">
          <span class="mr-1 text-xs text-[var(--text-muted)]"
            >{quickFacet.label}</span
          >
          {#each quickEntries as entry (entry)}
            {@const normalizedName = normalizeTaxonomyTagName(entry)}
            <button
              type="button"
              class="quick-filter"
              onclick={() => toggleTag(entry, quickFacet.entries)}
            >
              {entry}
              {#if counts[normalizedName] > 0}<span
                  class="tabular-nums text-[var(--text-muted)]"
                  >{counts[normalizedName]}</span
                >{/if}
            </button>
          {/each}
        </div>
      {:else}
        <p class="hidden truncate text-xs text-[var(--text-muted)] lg:block">
          {activeGroup.description}
        </p>
      {/if}

      <div class="ml-auto flex shrink-0 items-center gap-1.5">
        {#if activeTags.length > 0}
          <button
            type="button"
            class="min-h-8 rounded-md px-2 text-xs font-medium text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
            onclick={() => onchange([])}
          >
            清除
          </button>
        {/if}
        <button
          type="button"
          class="filter-toggle"
          aria-expanded={expanded}
          onclick={() => (expanded = !expanded)}
        >
          <SlidersHorizontal class="h-4 w-4" />
          <span>筛选{activeTags.length > 0 ? ` ${activeTags.length}` : ''}</span
          >
          <ChevronDown
            class="h-3.5 w-3.5 transition-transform {expanded
              ? 'rotate-180'
              : ''}"
          />
        </button>
      </div>
    </div>

    {#if expanded}
      <button
        type="button"
        class="filter-backdrop"
        aria-label="关闭筛选"
        onclick={() => (expanded = false)}
      ></button>
      <div class="taxonomy-panel">
        <div
          class="flex items-start justify-between gap-4 border-b border-[var(--border-subtle)] px-4 py-3 sm:px-5"
        >
          <div>
            <h3 class="text-sm font-semibold text-[var(--text-primary)]">
              {activeGroup.label}筛选
            </h3>
            <p class="mt-0.5 text-xs text-[var(--text-muted)]">
              {activeGroup.description}
            </p>
          </div>
          <button
            type="button"
            class="flex h-9 w-9 items-center justify-center rounded-md text-[var(--text-muted)] hover:bg-[var(--bg-hover)]"
            aria-label="关闭筛选"
            onclick={() => (expanded = false)}><X class="h-4 w-4" /></button
          >
        </div>

        {#if detailState && !detailState.selectedPrimary}
          <p
            class="border-b border-[var(--border-subtle)] bg-[var(--bg-stripe)] px-4 py-2.5 text-xs text-[var(--text-secondary)] sm:px-5"
          >
            先从左侧选择“{detailState.primaryFacet
              .label}”，再使用这里的专业细分。
          </p>
        {/if}

        <div
          class="grid gap-x-8 gap-y-5 overflow-y-auto px-4 py-4 sm:px-5 xl:grid-cols-2"
        >
          {#each detailState?.facets ?? [] as facet (facet.id)}
            <fieldset class="min-w-0">
              <legend
                class="mb-2 text-xs font-semibold tracking-wide text-[var(--text-secondary)]"
                >{facet.label}</legend
              >
              <div class="flex flex-wrap gap-1.5">
                {#each facet.entries as entry (entry)}
                  {@const normalizedName = normalizeTaxonomyTagName(entry)}
                  {@const selected = activeTags.some(
                    (tag) => normalizeTaxonomyTagName(tag) === normalizedName
                  )}
                  <button
                    type="button"
                    aria-pressed={selected}
                    class="facet-option"
                    class:selected
                    onclick={() => toggleTag(entry, facet.entries)}
                  >
                    {#if selected}<Check class="h-3.5 w-3.5 shrink-0" />{/if}
                    {entry}
                    {#if counts[normalizedName] > 0}<span
                        class="ml-0.5 tabular-nums opacity-65"
                        >{counts[normalizedName]}</span
                      >{/if}
                  </button>
                {/each}
              </div>
            </fieldset>
          {/each}
        </div>
      </div>
    {/if}
  </section>
{/if}

<style>
  .quick-filter,
  .selected-chip,
  .filter-toggle,
  .facet-option {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--border-default);
    border-radius: 0.3rem;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .quick-filter {
    min-height: 2rem;
    padding: 0.25rem 0.55rem;
    background: var(--bg-app);
    color: var(--text-secondary);
  }
  .quick-filter:hover,
  .facet-option:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .selected-chip {
    min-height: 1.75rem;
    padding: 0.2rem 0.45rem;
    border-color: color-mix(in oklab, var(--accent) 45%, var(--border-default));
    background: var(--accent-soft);
    color: var(--accent-text);
  }
  .filter-toggle {
    min-height: 2.25rem;
    padding: 0.35rem 0.6rem;
    background: var(--selection-bar-bg);
    color: var(--selection-bar-text);
  }
  .taxonomy-panel {
    position: relative;
    z-index: 30;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-surface);
    box-shadow: 0 12px 30px rgb(24 30 27 / 0.12);
  }
  .facet-option {
    min-height: 2rem;
    padding: 0.3rem 0.6rem;
    background: var(--bg-elevated);
    color: var(--text-secondary);
  }
  .facet-option.selected {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    box-shadow: inset 0 0 0 1px
      color-mix(in oklab, var(--accent) 25%, transparent);
  }
  .filter-backdrop {
    display: none;
  }
  @media (max-width: 639px) {
    .filter-backdrop {
      position: fixed;
      inset: 0;
      z-index: 49;
      display: block;
      width: 100%;
      border: 0;
      background: rgb(16 23 20 / 0.42);
    }
    .taxonomy-panel {
      position: fixed;
      right: 0;
      bottom: calc(64px + env(safe-area-inset-bottom));
      left: 0;
      z-index: 50;
      max-height: 72dvh;
      overflow: hidden;
      border-top: 1px solid var(--border-default);
      border-radius: 0.75rem 0.75rem 0 0;
    }
    .taxonomy-panel > div:last-child {
      max-height: calc(72dvh - 72px);
    }
  }
</style>
