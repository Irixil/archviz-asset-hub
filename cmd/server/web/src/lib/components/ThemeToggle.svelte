<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { Moon, Sun } from '@lucide/svelte'
  import { themeStore } from '$lib/stores/theme.svelte'
</script>

<button
  onclick={() => themeStore.toggle()}
  aria-label={themeStore.dark ? m.theme_switch_light() : m.theme_switch_dark()}
  class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs font-medium text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] sm:px-2.5"
>
  {#if themeStore.dark}
    <Sun class="h-3.5 w-3.5 shrink-0" />
    <span class="hidden sm:inline">{m.theme_light()}</span>
  {:else}
    <Moon class="h-3.5 w-3.5 shrink-0" />
    <span class="hidden sm:inline">{m.theme_dark()}</span>
  {/if}
</button>
