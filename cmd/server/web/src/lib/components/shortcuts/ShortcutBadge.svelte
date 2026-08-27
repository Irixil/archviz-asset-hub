<script lang="ts">
  import { formatKey } from '$lib/shortcuts'
  import { DEFAULT_KEYMAP } from '$lib/shortcuts'
  import { keymap } from '$lib/shortcuts/store.svelte'
  import type { ShortcutAction } from '$lib/shortcuts'

  interface Props {
    action: ShortcutAction
    index?: number
  }

  let { action, index = 0 }: Props = $props()

  const combos = $derived(
    keymap.current[action] ?? DEFAULT_KEYMAP[action] ?? []
  )
  const combo = $derived(combos[index] ?? '')
  const keys = $derived(combo.split('+').map(formatKey))
</script>

{#if combo}
  <span
    class="inline-flex items-center gap-0.5 font-mono text-xs opacity-60"
    aria-label="Keyboard shortcut: {combo}"
  >
    {#each keys as key, i}
      <kbd
        class="inline-block rounded border border-zinc-300 bg-zinc-100 px-1 py-px font-mono text-xs leading-tight text-zinc-700 dark:border-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
        >{key}</kbd
      >{#if i < keys.length - 1}<span class="text-zinc-400">+</span>{/if}
    {/each}
  </span>
{/if}
