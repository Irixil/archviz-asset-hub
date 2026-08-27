<script lang="ts">
  import { tagApi, type Tag } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'

  interface Props {
    activeTags: string[]
    onchange: (tags: string[]) => void
  }

  let { activeTags, onchange }: Props = $props()

  let allTags = $state<Tag[]>([])

  async function loadTags() {
    try {
      const hideEmpty = authStore.taxonomyLocked && !authStore.canCreateTag
      allTags = await tagApi.list({ hide_empty: hideEmpty })
    } catch {
      // silently ignore
    }
  }

  $effect(() => {
    loadTags()
  })

  function toggle(name: string, event: MouseEvent) {
    if (event.shiftKey) {
      // Multi-select AND
      if (activeTags.includes(name)) {
        onchange(activeTags.filter((t) => t !== name))
      } else {
        onchange([...activeTags, name])
      }
    } else {
      // Single select — toggle off if already the only active
      if (activeTags.length === 1 && activeTags[0] === name) {
        onchange([])
      } else {
        onchange([name])
      }
    }
  }

  function dismiss(name: string) {
    onchange(activeTags.filter((t) => t !== name))
  }
</script>

{#if allTags.length > 0}
  <!-- Active chips -->
  {#each activeTags as tag}
    <Chip label={tag} onremove={() => dismiss(tag)} color="#4f46e5" />
  {/each}

  {#if activeTags.length > 0}
    <span class="h-4 w-px shrink-0 bg-[var(--border-default)]"></span>
  {/if}

  <!-- All available tags -->
  {#each allTags as tag}
    {#if !activeTags.includes(tag.name)}
      <button
        class="rounded-full border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-1 text-xs font-medium whitespace-nowrap text-[var(--text-secondary)] transition-colors hover:border-[var(--accent)] hover:bg-[var(--accent-soft)] hover:text-[var(--accent-text)]"
        onclick={(e) => toggle(tag.name, e)}
        title="按住 Shift 可叠加筛选"
      >
        {tag.name}
        <span class="ml-1 text-[var(--text-muted)] tabular-nums"
          >{tag.asset_count}</span
        >
      </button>
    {/if}
  {/each}

  {#if activeTags.length > 0}
    <button
      class="rounded px-1.5 py-0.5 text-xs text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
      onclick={() => onchange([])}>清除</button
    >
  {/if}
{/if}
