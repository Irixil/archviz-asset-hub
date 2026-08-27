<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import DuplicateBadge from './DuplicateBadge.svelte'
  import type { DuplicateOf } from '$lib/api/assets'

  interface Props {
    duplicate: DuplicateOf
    onKeepBoth: () => void
    onDeleteThisCopy: () => Promise<void>
  }

  let { duplicate, onKeepBoth, onDeleteThisCopy }: Props = $props()

  let showConfirm = $state(false)
  let deleting = $state(false)

  async function handleConfirmDelete() {
    deleting = true
    try {
      await onDeleteThisCopy()
    } finally {
      deleting = false
    }
  }
</script>

<div
  role="status"
  class="flex flex-wrap items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm dark:border-amber-800 dark:bg-amber-900/20"
>
  <DuplicateBadge
    isDeletedVersion={duplicate.is_deleted_version}
    storageAvailable={duplicate.storage_available}
  />
  <span class="flex-1 text-amber-800 dark:text-amber-300">
    {duplicate.is_deleted_version
      ? m.duplicate_banner_matches_deleted({
          filename: duplicate.original_filename,
        })
      : m.duplicate_banner_matches_existing({
          filename: duplicate.original_filename,
        })}
  </span>

  {#if duplicate.storage_available}
    <a
      href="/library/assets/{duplicate.asset_id}"
      class="font-medium text-amber-800 underline hover:no-underline dark:text-amber-300"
    >
      {m.duplicate_banner_view_existing()}
    </a>
  {/if}

  <Button variant="secondary" size="sm" onclick={onKeepBoth}>
    {m.duplicate_banner_keep_both()}
  </Button>
  <Button
    variant="danger"
    size="sm"
    disabled={deleting}
    loading={deleting}
    onclick={() => (showConfirm = true)}
  >
    {m.duplicate_banner_delete_copy()}
  </Button>
</div>

{#if showConfirm}
  <ConfirmModal
    open={showConfirm}
    title={m.duplicate_banner_confirm_title()}
    message={m.duplicate_banner_confirm_message()}
    confirmLabel={m.duplicate_banner_delete_copy()}
    onConfirm={handleConfirmDelete}
    onCancel={() => (showConfirm = false)}
  />
{/if}
