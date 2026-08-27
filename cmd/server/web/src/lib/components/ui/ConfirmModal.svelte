<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    open?: boolean
    title: string
    message?: string
    items?: string[]
    confirmLabel?: string
    onConfirm: () => void
    onCancel: () => void
  }

  let {
    open = $bindable(false),
    title,
    message,
    items = [],
    confirmLabel,
    onConfirm,
    onCancel,
  }: Props = $props()

  const MAX_SHOWN = 5
  const shown = $derived(items.slice(0, MAX_SHOWN))
  const overflow = $derived(items.length - MAX_SHOWN)

  function handleConfirm() {
    open = false
    onConfirm()
  }

  function handleCancel() {
    open = false
    onCancel()
  }
</script>

<Modal {open} onclose={handleCancel}>
  <div class="px-6 py-5">
    <h2 class="text-base font-semibold text-[var(--text-primary)]">{title}</h2>

    {#if items.length > 0}
      <ul
        class="mt-3 space-y-1 rounded-lg border border-[var(--border-default)] bg-[var(--bg-default)] px-3 py-2"
      >
        {#each shown as item}
          <li class="truncate text-sm text-[var(--text-secondary)]">{item}</li>
        {/each}
        {#if overflow > 0}
          <li class="text-sm text-[var(--text-muted)]">+ {overflow} more</li>
        {/if}
      </ul>
    {/if}

    {#if message}
      <p class="mt-3 text-sm text-[var(--text-secondary)]">{message}</p>
    {/if}

    <p class="mt-2 text-sm font-medium text-red-600 dark:text-red-400">
      {m.cannot_be_undone()}
    </p>

    <div class="mt-5 flex justify-end gap-2">
      <Button variant="secondary" size="sm" onclick={handleCancel}>
        {m.cancel()}
      </Button>
      <Button variant="danger" size="sm" onclick={handleConfirm}>
        {confirmLabel ?? m.delete_permanently()}
      </Button>
    </div>
  </div>
</Modal>
