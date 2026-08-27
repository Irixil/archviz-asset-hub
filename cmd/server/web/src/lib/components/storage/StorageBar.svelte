<script lang="ts">
  import { formatBytes } from '$lib/api/client'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    height?: number
    used: number
    limit: number | null
    compact?: boolean
  }

  let { height = 4, used, limit, compact = false }: Props = $props()

  const pct = $derived(
    limit != null && limit > 0 ? Math.min((used / limit) * 100, 100) : null
  )

  const color = $derived(
    pct == null
      ? 'var(--accent-success)'
      : pct > 95
        ? 'var(--accent-error, #ef4444)'
        : pct > 80
          ? 'var(--accent-warning, #f59e0b)'
          : 'var(--accent-success, #22c55e)'
  )

  const label = $derived(
    limit != null
      ? m.storage_used_of({
          used: formatBytes(used, 0),
          limit: formatBytes(limit, 0),
        })
      : m.storage_used({ bytes: formatBytes(used, 0) })
  )
</script>

<div class="storage-bar" title={compact ? label : undefined}>
  <div class="bar-track" style="height: {height}px">
    <div
      class="bar-fill"
      style="width: {pct != null ? pct : 5}%; background-color: {color};"
    ></div>
  </div>
  <span class="bar-label">{label}</span>
</div>

<style>
  .storage-bar {
    display: flex;
    flex-direction: column;
    text-align: end;
    gap: 4px;
  }
  .bar-track {
    border-radius: 2px;
    background: var(--bg-subtle, rgba(128, 128, 128, 0.2));
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    border-radius: 2px;
    transition: width 0.3s ease;
  }
  .bar-label {
    font-size: 0.7rem;
    color: var(--text-muted);
    line-height: 1;
  }
</style>
