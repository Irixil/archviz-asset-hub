<script lang="ts">
  import Hint from './Hint.svelte'
  import Title from './Title.svelte'
  import UserMenu from './UserMenu.svelte'

  let {
    title,
    description = undefined,
    meta = undefined,
    children = undefined,
  } = $props()
</script>

<!-- Header -->
<header
  class="border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-6 py-4"
>
  <div class="flex items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <Title>{title}</Title>
      {#if description}
        <Hint>{description}</Hint>
      {/if}
      {#if meta}
        <div class="mt-2">
          {@render meta()}
        </div>
      {/if}
    </div>
    <div class="flex shrink-0 items-center gap-3">
      {@render children?.()}
      <UserMenu />
    </div>
  </div>
</header>
