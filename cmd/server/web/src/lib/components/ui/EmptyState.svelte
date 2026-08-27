<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    title: string
    description?: string
    icon?: Snippet
    action?: Snippet
  }

  let { title, description, icon, action }: Props = $props()
</script>

<div class="flex flex-col items-center justify-center py-20 text-center">
  {#if icon}
    <div class="mb-4 text-[var(--text-muted)] opacity-45">
      {@render icon()}
    </div>
  {/if}
  <p class="text-sm font-semibold text-[var(--text-primary)]">{title}</p>
  {#if description}
    <p class="mt-1 text-sm text-[var(--text-muted)]">{description}</p>
  {/if}
  {#if action}
    <div class="mt-4">
      {@render action()}
    </div>
  {/if}
</div>
