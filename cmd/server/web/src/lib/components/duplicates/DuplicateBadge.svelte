<script lang="ts">
  import Badge from '$lib/components/ui/Badge.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    isDeletedVersion: boolean
    storageAvailable: boolean
  }

  let { isDeletedVersion, storageAvailable }: Props = $props()
</script>

{#if isDeletedVersion}
  <Badge variant="neutral">{m.duplicate_badge_deleted()}</Badge>
{:else if !storageAvailable}
  <Badge variant="neutral">{m.duplicate_badge_unavailable()}</Badge>
{:else}
  <Badge variant="neutral">{m.duplicate_badge_duplicate()}</Badge>
{/if}
