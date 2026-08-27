<script lang="ts">
  import { useShortcut, SHORTCUT_GROUPS, formatKey } from '$lib/shortcuts'
  import { keymap } from '$lib/shortcuts/store.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import { m } from '$lib/paraglide/messages'

  let open = $state(false)

  useShortcut('help.toggle', () => {
    open = !open
  })

  function getKeys(action: string): string[] {
    return (keymap.current as Record<string, string[]>)[action] ?? []
  }

  function formatCombo(combo: string): string {
    return combo
      .split(' ')
      .map((part) => part.split('+').map(formatKey).join('+'))
      .join(' → ')
  }
</script>

<Modal
  bind:open
  onclose={() => {
    open = false
  }}
>
  <div class="p-6">
    <h2 class="mb-6 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
      {m.keyboard_shortcuts()}
    </h2>

    <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
      {#each SHORTCUT_GROUPS as group}
        <div>
          <h3
            class="mb-2 text-xs font-semibold tracking-widest text-zinc-500 uppercase dark:text-zinc-400"
          >
            {group.title}
          </h3>
          <ul class="space-y-1">
            {#each group.actions as item}
              {@const combos = getKeys(item.action)}
              {#if combos.length > 0}
                <li
                  class="flex items-center justify-between gap-4 text-zinc-700 dark:text-zinc-300"
                >
                  <span class="text-sm">{item.label}</span>
                  <span class="flex shrink-0 items-center gap-1">
                    {#each combos.slice(0, 2) as combo, i}
                      {#if i > 0}<span class="text-xs text-zinc-400">or</span
                        >{/if}
                      <kbd
                        class="inline-block rounded border border-zinc-300 bg-zinc-100 px-1.5 py-0.5 font-mono text-xs text-zinc-700 dark:border-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                        >{formatCombo(combo)}</kbd
                      >
                    {/each}
                  </span>
                </li>
              {/if}
            {/each}
          </ul>
        </div>
      {/each}
    </div>

    <p class="mt-6 text-xs text-zinc-400 italic dark:text-zinc-500">
      {m.shortcuts_contextual_hint()}
    </p>
  </div>
</Modal>
