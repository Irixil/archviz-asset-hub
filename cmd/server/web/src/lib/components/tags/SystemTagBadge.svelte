<script lang="ts">
  import { Lock } from '@lucide/svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import { m } from '$lib/paraglide/messages'
</script>

<span title={m.tags_system_tag_badge_tooltip()}>
  <Badge variant="neutral">
    {#snippet children()}
      <span class="inline-flex items-center gap-1">
        <Lock class="h-3 w-3" />
        {m.tags_system_group_label()}
      </span>
    {/snippet}
  </Badge>
</span>
