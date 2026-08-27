<script lang="ts" module>
  export const panelTabs = {
    details: 'details',
    tags: 'tags',
    variants: 'variants',
    text: 'text',
    comments: 'comments',
    history: 'history',
    activity: 'activity',
    actions: 'actions',
  } as const

  export type PanelTab = keyof typeof panelTabs
</script>

<script lang="ts">
  import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    activeTab: PanelTab
    asset: Asset
    isMediaTagsAsset: boolean
    onTabChange: (tab: PanelTab) => void
  }

  let { activeTab, asset, isMediaTagsAsset, onTabChange }: Props = $props()

  const tabLabels: Record<PanelTab, string> = $derived({
    details: m.tab_details(),
    tags: m.media_tags_tab_label(),
    variants: m.tab_variants(),
    text: m.text_tracks_panel_title(),
    comments: m.tab_comments(),
    history: m.tab_history(),
    activity: m.tab_activity(),
    actions: m.tab_actions(),
  })
</script>

<div
  class="relative flex-shrink-0 border-b border-gray-100 dark:border-gray-800"
>
  <div
    class="flex overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
  >
    {#each Object.keys(panelTabs) as tab}
      {@const key = tab as PanelTab}
      {#if tab !== 'tags' || isMediaTagsAsset}
        <button
          type="button"
          class="relative min-w-fit flex-1 px-2 py-2.5 text-xs font-medium whitespace-nowrap transition-colors {activeTab ===
          tab
            ? 'text-indigo-600 dark:text-indigo-400'
            : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'}"
          onclick={() => onTabChange(key)}
        >
          {#if tabLabels[key]}
            {tabLabels[key].charAt(0).toUpperCase() + tabLabels[key].slice(1)}
          {/if}
          {#if tab === 'history' && (asset?.version_count ?? 0) > 1}
            <span
              class="ml-1 rounded-full bg-indigo-100 px-1.5 py-0.5 text-xs font-medium text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400"
            >
              {asset.version_count}
            </span>
          {/if}
          {#if activeTab === tab}
            <span
              class="absolute right-0 bottom-0 left-0 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"
            ></span>
          {/if}
        </button>
      {/if}
    {/each}
  </div>
</div>
