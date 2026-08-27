<script lang="ts">
  const API_BASE = import.meta.env.VITE_API_URL ?? ''
  import { Bot } from '@lucide/svelte'
  let {
    actor = null,
    class: extraClass,
  }: {
    actor: { id?: string | null; name?: string | null } | null
    class?: string
  } = $props()
</script>

{#if actor?.id}
  <img
    src={`${API_BASE}/api/v1/users/${actor.id}/avatar`}
    alt={actor.name || actor.id.split('-')[0]}
    class="rounded-full object-cover {extraClass}"
  />
{:else}
  <Bot
    class="bg-zinc-100 text-zinc-400 dark:bg-zinc-800 dark:text-zinc-500 {extraClass}"
  />
{/if}
