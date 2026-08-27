<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { exportsApi, type ExportRun } from '$lib/api/exports'

  interface Props {
    runId: string
    onDone?: (run: ExportRun) => void
  }

  let { runId, onDone }: Props = $props()

  let run = $state<ExportRun | null>(null)
  let interval: ReturnType<typeof setInterval> | null = null
  let errorCount = 0
  const MAX_ERRORS = 5

  function formatBytes(n: number) {
    if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
    if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
    if (n >= 1e3) return (n / 1e3).toFixed(0) + ' KB'
    return n + ' B'
  }

  async function poll() {
    try {
      run = await exportsApi.getRun(runId)
      errorCount = 0
      if (run.status === 'done' || run.status === 'failed') {
        stopPolling()
        onDone?.(run)
      }
    } catch {
      if (++errorCount >= MAX_ERRORS) {
        stopPolling()
        onDone?.({ id: runId, status: 'failed' } as ExportRun)
      }
    }
  }

  function stopPolling() {
    if (interval) {
      clearInterval(interval)
      interval = null
    }
  }

  onMount(() => {
    poll()
    interval = setInterval(poll, 3000)
  })

  onDestroy(stopPolling)

  const pct = $derived(
    run && run.assets_total > 0
      ? Math.round((run.assets_exported / run.assets_total) * 100)
      : 0
  )

  const statusClass = $derived(
    run?.status === 'done'
      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
      : run?.status === 'failed'
        ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
        : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'
  )
</script>

{#if run}
  <div class="space-y-2">
    <div class="flex items-center justify-between text-sm">
      <span class="text-gray-500 dark:text-gray-400">
        已写入 {run.assets_exported}/{run.assets_total} 个文件 · {formatBytes(
          run.bytes_written
        )}
        {#if run.assets_skipped > 0}
          · 跳过 {run.assets_skipped} 个
        {/if}
      </span>
      <span class="rounded-full px-2 py-0.5 text-xs font-medium {statusClass}">
        {run.status === 'done'
          ? '已完成'
          : run.status === 'failed'
            ? '失败'
            : '进行中'}
      </span>
    </div>

    <div
      class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700"
    >
      <div
        class="h-full rounded-full transition-all duration-500 {run.status ===
        'failed'
          ? 'bg-red-500'
          : 'bg-indigo-500'}"
        style="width: {pct}%"
      ></div>
    </div>

    {#if run.error}
      <p class="text-xs text-red-600 dark:text-red-400">{run.error}</p>
    {/if}
  </div>
{:else}
  <div
    class="h-1.5 w-full animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"
  ></div>
{/if}
