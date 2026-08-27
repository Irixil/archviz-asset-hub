<script lang="ts">
  import { formatDistanceToNow } from 'date-fns'
  import { zhCN } from 'date-fns/locale'
  import { Server, FolderOpen, Clock } from '@lucide/svelte'
  import {
    exportsApi,
    type ExportConfig,
    type ExportRun,
  } from '$lib/api/exports'
  import ExportRunProgress from './ExportRunProgress.svelte'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import SectionHeading from '$lib/components/ui/SectionHeading.svelte'
  import ButtonStart from '$lib/components/ui/ButtonStart.svelte'
  import ButtonEdit from '$lib/components/ui/ButtonEdit.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ButtonToggle from '$lib/components/ui/ButtonToggle.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'

  interface Props {
    config: ExportConfig
    onEdit: (config: ExportConfig) => void
    onDelete: (config: ExportConfig) => void
    onToggle?: (config: ExportConfig) => void
  }

  let { config, onEdit, onDelete, onToggle }: Props = $props()

  let activeRunId = $state<string | null>(null)
  let triggering = $state(false)

  async function trigger() {
    triggering = true
    try {
      const run = await exportsApi.trigger(config.id)
      activeRunId = run.id
    } catch {
      // handled by parent
    } finally {
      triggering = false
    }
  }

  async function onRunDone(_run: ExportRun) {
    activeRunId = null
    try {
      config = await exportsApi.get(config.id)
    } catch {
      // best-effort refresh
    }
  }

  const runStatus = $derived(
    config.last_run_status === 'ok'
      ? 'healthy'
      : config.last_run_status === 'failed'
        ? 'error'
        : ('disabled' as 'healthy' | 'error' | 'disabled')
  )

  const runStatusText = $derived(
    config.last_run_status === 'ok'
      ? 'OK'
      : config.last_run_status === 'partial'
        ? '部分完成'
        : config.last_run_status === 'failed'
          ? '失败'
          : config.last_run_status === 'pending'
            ? '等待中'
            : (config.last_run_status ?? '')
  )

  const scheduleLabel = $derived(
    config.schedule_type === 'after_quiet' && config.quiet_minutes
      ? `连续 ${config.quiet_minutes >= 60 ? config.quiet_minutes / 60 + ' 小时' : config.quiet_minutes + ' 分钟'}无新上传后执行`
      : '手动执行'
  )

  const destTypeLabel = $derived(
    config.dest_type === 'gdrive' ? 'Google Drive' : 'SFTP'
  )

  const destProject = $derived(
    projectsStore.projects.find((p) => p.id === config.project_id) ?? null
  )
</script>

<div
  class="overflow-hidden rounded-xl border border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
>
  <div class="flex items-start gap-4 px-5 py-4">
    <!-- Icon -->
    <div
      class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-500 dark:bg-indigo-900/30 dark:text-indigo-400"
    >
      {#if config.dest_type === 'gdrive'}
        <FolderOpen class="h-5 w-5" />
      {:else}
        <Server class="h-5 w-5" />
      {/if}
    </div>

    <!-- Meta -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <SectionHeading title={config.label} />
        <Hint>{destTypeLabel}</Hint>
        <StatusBadge
          status={!config.enabled
            ? 'disabled'
            : config.last_run_status === 'failed'
              ? 'error'
              : 'healthy'}
        />
      </div>

      <!-- Destination project + schedule metadata -->
      <div
        class="mt-1 flex flex-wrap items-center gap-3 text-sm text-gray-400 dark:text-gray-500"
      >
        {#if destProject}
          <span>→ {destProject.name}</span>
        {/if}
        <span class="inline-flex items-center gap-1">
          <Clock class="h-3 w-3" />
          {#if config.last_run_at}
            {formatDistanceToNow(new Date(config.last_run_at), {
              addSuffix: true,
              locale: zhCN,
            })}
          {:else}
            从未执行
          {/if}
        </span>
        <span>{scheduleLabel}</span>
        {#if config.versions === 'all'}
          <span>全部版本</span>
        {/if}
        {#if config.include_variants}
          <span>包含衍生版本</span>
        {/if}
      </div>

      <!-- Run progress or last run status -->
      {#if activeRunId}
        <div class="mt-3">
          <ExportRunProgress runId={activeRunId} onDone={onRunDone} />
        </div>
      {:else if config.last_run_at && config.last_run_status && config.last_run_status !== 'ok'}
        <div class="mt-2">
          <StatusBadge status={runStatus} text={runStatusText} />
        </div>
      {/if}

      <!-- Error message -->
      {#if config.last_error && config.last_run_status === 'failed'}
        <div
          class="mt-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300"
        >
          {config.last_error}
        </div>
      {/if}
    </div>

    <!-- Actions -->
    <div class="flex shrink-0 items-center gap-1">
      {#if !activeRunId}
        <ButtonStart
          onclick={trigger}
          disabled={triggering || !config.enabled}
          title="立即执行"
        />
      {/if}
      {#if onToggle}
        <ButtonToggle
          enabled={config.enabled}
          onclick={() => onToggle?.(config)}
          title={config.enabled ? '停用' : '启用'}
        />
      {/if}
      <ButtonEdit onclick={() => onEdit(config)} />
      <ButtonDelete onclick={() => onDelete(config)} />
    </div>
  </div>
</div>
