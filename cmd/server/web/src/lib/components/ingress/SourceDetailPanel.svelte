<script lang="ts">
  import { onMount } from 'svelte'
  import { X, Save, RefreshCw, RotateCcw, ExternalLink } from '@lucide/svelte'
  import type { IngressSource, IngressSourceType } from '$lib/api'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import SourceConfigForm from './SourceConfigForm.svelte'
  import EmailApiPanel from './EmailApiPanel.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import GridSkeleton from '../ui/GridSkeleton.svelte'
  import Hint from '../ui/Hint.svelte'
  import Feedback from '../ui/Feedback.svelte'
  import ButtonDelete from '../ui/ButtonDelete.svelte'
  import { m } from '$lib/paraglide/messages'
  import { fly } from 'svelte/transition'

  interface Props {
    source: IngressSource
    onclose: () => void
    onupdated: (source: IngressSource) => void
  }

  let { source, onclose, onupdated }: Props = $props()

  type Tab = 'config' | 'rules' | 'log'
  let activeTab = $state<Tab>('config')

  const TAB_LABELS: Record<Tab, string> = {
    config: '连接设置',
    rules: '整理规则',
    log: '导入记录',
  }
  const SOURCE_LABELS: Record<string, string> = {
    email_api: m.ingress_own_mail(),
    imap: m.ingress_imap(),
    sftp: m.ingress_sftp(),
    dav: m.ingress_dav(),
    s3: m.ingress_s3(),
    gdrive: m.ingress_gdrive_label(),
    canva: m.ingress_canva_label(),
  }

  // --- Config tab ---
  // svelte-ignore state_referenced_locally
  let src = $state(source)
  let label = $state(src.label)
  let destProjectId = $state(src.dest_project_id ?? '')
  let pollIntervalMin = $state(src.poll_interval_min)
  let sourceConfig = $state<Record<string, unknown>>({
    ...(src.config as Record<string, unknown>),
  })
  let saving = $state(false)

  onMount(() => {
    if (src.type === 'email_api' && src.dest_project_id) {
      foldersStore.loadForProject(src.dest_project_id)
    }
  })

  const emailApiFolders = $derived(
    src.type === 'email_api' && src.dest_project_id
      ? (foldersStore.foldersByProject[src.dest_project_id] ?? [])
      : []
  )

  const POLL_INTERVALS = [
    { label: '5 分钟', value: 5 },
    { label: '15 分钟', value: 15 },
    { label: '30 分钟', value: 30 },
    { label: '1 小时', value: 60 },
    { label: '6 小时', value: 360 },
  ]

  async function saveConfig() {
    saving = true
    const updated = await ingressStore.updateSource(src.id, {
      label: label.trim() || src.label,
      config: sourceConfig,
      dest_project_id: destProjectId || undefined,
      poll_interval_min: pollIntervalMin,
    })
    if (updated) onupdated(updated)
    saving = false
  }

  // --- Log tab ---
  let logFilter = $state<string>('')

  const filteredLog = $derived(
    logFilter
      ? ingressStore.log.filter((e) => e.status === logFilter)
      : ingressStore.log
  )

  const STATUS_STYLES: Record<string, string> = {
    imported:
      'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    pending:
      'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    skipped: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleString('zh-CN', {
      dateStyle: 'short',
      timeStyle: 'short',
    })
  }

  // --- Rules tab ---
  const FIELD_OPTIONS = [
    'mime_type',
    'filename',
    'sender',
    'size_bytes',
    'subject',
  ]
  const ACTION_OPTIONS = ['allow', 'deny', 'route_to_folder']
  const FIELD_LABELS: Record<string, string> = {
    mime_type: '文件类型',
    filename: '文件名',
    sender: '发送人',
    size_bytes: '文件大小',
    subject: '邮件主题',
  }
  const OPERATOR_LABELS: Record<string, string> = {
    equals: '等于',
    starts_with: '开头为',
    contains: '包含',
    ends_with: '结尾为',
    gt: '大于',
    lt: '小于',
  }
  const ACTION_LABELS: Record<string, string> = {
    allow: '允许导入',
    deny: '不导入',
    route_to_folder: '归入文件夹',
  }
  const STATUS_LABELS: Record<string, string> = {
    imported: '已导入',
    pending: '等待中',
    failed: '失败',
    skipped: '已跳过',
  }
  const OPERATOR_MAP: Record<string, string[]> = {
    mime_type: ['equals', 'starts_with'],
    filename: ['contains', 'ends_with', 'equals'],
    sender: ['equals', 'contains'],
    size_bytes: ['gt', 'lt'],
    subject: ['contains'],
  }

  let newRule = $state({
    field: 'filename',
    operator: 'contains',
    value: '',
    action: 'allow',
  })
  let addingRule = $state(false)
  let savingRule = $state(false)

  const availableOperators = $derived(OPERATOR_MAP[newRule.field] ?? ['equals'])

  async function handleAddRule() {
    if (!newRule.value.trim()) return
    savingRule = true
    const rule = await ingressStore.createRule(src.id, {
      position: ingressStore.rules.length,
      field: newRule.field,
      operator: newRule.operator,
      value: newRule.value,
      action: newRule.action,
    })
    if (rule) {
      newRule = {
        field: 'filename',
        operator: 'contains',
        value: '',
        action: 'allow',
      }
      addingRule = false
    }
    savingRule = false
  }

  // Load log + rules when switching tabs
  $effect(() => {
    if (activeTab === 'log' && ingressStore.logSourceId !== src.id) {
      ingressStore.loadLog(src.id)
    }
    if (activeTab === 'rules' && ingressStore.rulesSourceId !== src.id) {
      ingressStore.loadRules(src.id)
    }
  })
</script>

<aside
  in:fly={{ x: 250, duration: 150 }}
  class="flex w-2xl shrink-0 flex-col border-l border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
>
  <!-- Panel header -->
  <div
    class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800"
  >
    <div class="min-w-0">
      <h2
        class="text-md truncate font-semibold text-gray-900 dark:text-gray-50"
      >
        {src.label}
      </h2>
      <Hint class="text-sm">{SOURCE_LABELS[src.type] ?? src.type}</Hint>
    </div>
    <button
      type="button"
      class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
      onclick={onclose}
    >
      <X class="h-4 w-4" />
    </button>
  </div>

  <Feedback class="bg-transparent" error={src.last_error} />

  <!-- Tabs -->
  <div class="flex border-b border-gray-100 dark:border-gray-800">
    {#each ['config', 'rules', 'log'] as Tab[] as tab}
      <button
        type="button"
        class="flex-1 py-2.5 text-sm font-medium capitalize transition-colors
          {activeTab === tab
          ? 'border-b-2 border-indigo-500 text-indigo-600 dark:text-indigo-400'
          : 'text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}"
        onclick={() => {
          activeTab = tab
        }}
      >
        {TAB_LABELS[tab]}
      </button>
    {/each}
  </div>

  <!-- Tab content -->
  <div class="flex-1 overflow-y-auto">
    <!-- CONFIG TAB -->
    {#if activeTab === 'config'}
      {#if src.type === 'email_api'}
        <div class="p-5">
          <EmailApiPanel {source} folders={emailApiFolders} />
        </div>
      {:else}
        <div class="space-y-5 p-5">
          <!-- Label -->
          <div>
            <label
              for="detail-label"
              class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
              >名称</label
            >
            <input
              id="detail-label"
              type="text"
              bind:value={label}
              class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            />
          </div>

          <!-- Destination project -->
          <div>
            <label
              for="detail-project"
              class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
              >{m.dest_project()}</label
            >
            <select
              id="detail-project"
              bind:value={destProjectId}
              class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            >
              <option value="">{m.none_choice()}</option>
              {#each projectsStore.projects as p (p.id)}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          </div>

          <!-- Poll interval -->
          <div>
            <label
              for="detail-interval"
              class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
              >{m.poll_interval()}</label
            >
            <select
              id="detail-interval"
              bind:value={pollIntervalMin}
              class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            >
              {#each POLL_INTERVALS as opt (opt.value)}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </div>

          <!-- Source-specific config -->
          <div class="border-t border-gray-100 pt-4 dark:border-gray-800">
            <p
              class="mb-3 text-sm font-semibold tracking-widest text-gray-400 uppercase"
            >
              连接设置
            </p>
            <SourceConfigForm
              type={src.type as IngressSourceType}
              bind:config={sourceConfig}
            />
          </div>
        </div>

        <!-- Save button -->
        <div class="border-t border-gray-100 px-5 py-4 dark:border-gray-800">
          <Button
            variant="primary"
            size="sm"
            loading={saving}
            onclick={saveConfig}
          >
            {#snippet icon()}<Save class="h-3.5 w-3.5" />{/snippet}
            {m.save()}
          </Button>
        </div>
      {/if}

      <!-- RULES TAB -->
    {:else if activeTab === 'rules'}
      <div class="p-5">
        {#if ingressStore.loadingRules}
          <GridSkeleton lines={3} />
        {:else if ingressStore.rules.length === 0 && !addingRule}
          <div class="py-10 text-center">
            <Hint>{m.no_rules_yet()}</Hint>
            <button
              type="button"
              class="mt-3 text-sm font-medium text-indigo-600 hover:underline dark:text-indigo-400"
              onclick={() => {
                addingRule = true
              }}
            >
              {m.add_first_rule()}
            </button>
          </div>
        {:else}
          <div class="space-y-2">
            {#each ingressStore.rules as rule (rule.id)}
              <div
                class="flex items-center gap-2 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50"
              >
                <div class="min-w-0 flex-1">
                  <p class="text-sm text-gray-700 dark:text-gray-300">
                    <span class="font-medium"
                      >{FIELD_LABELS[rule.field] ?? rule.field}</span
                    >
                    <span class="text-gray-400">
                      {OPERATOR_LABELS[rule.operator] ?? rule.operator}
                    </span>
                    <span class="font-mono text-gray-600 dark:text-gray-200"
                      >"{rule.value}"</span
                    >
                    <span class="text-gray-400"> → </span>
                    <span
                      class="font-medium text-indigo-600 dark:text-indigo-400"
                      >{ACTION_LABELS[rule.action] ?? rule.action}</span
                    >
                  </p>
                </div>
                <ButtonDelete
                  title={m.delete_rule()}
                  onclick={() => ingressStore.deleteRule(src.id, rule.id)}
                />
              </div>
            {/each}
          </div>
        {/if}

        <!-- Add rule form -->
        {#if addingRule}
          <div
            class="mt-4 space-y-3 rounded-lg border border-indigo-200 bg-indigo-50/40 p-4 dark:border-indigo-800 dark:bg-indigo-900/20"
          >
            <p class="text-sm font-semibold text-gray-600 dark:text-gray-300">
              新建规则
            </p>
            <div class="grid grid-cols-2 gap-2">
              <select
                bind:value={newRule.field}
                onchange={() => {
                  newRule.operator = (OPERATOR_MAP[newRule.field] ?? [
                    'equals',
                  ])[0]
                }}
                class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-indigo-300 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              >
                {#each FIELD_OPTIONS as f}
                  <option value={f}>{FIELD_LABELS[f] ?? f}</option>
                {/each}
              </select>
              <select
                bind:value={newRule.operator}
                class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-indigo-300 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
              >
                {#each availableOperators as op}
                  <option value={op}>{OPERATOR_LABELS[op] ?? op}</option>
                {/each}
              </select>
            </div>
            <input
              type="text"
              placeholder="值"
              bind:value={newRule.value}
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-indigo-400 focus:ring-1 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            />
            <select
              bind:value={newRule.action}
              class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 focus:ring-1 focus:ring-indigo-300 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            >
              {#each ACTION_OPTIONS as a}
                <option value={a}>{ACTION_LABELS[a] ?? a}</option>
              {/each}
            </select>
            <div class="flex gap-2">
              <Button
                variant="primary"
                size="sm"
                loading={savingRule}
                onclick={handleAddRule}>{m.add_rule()}</Button
              >
              <Button
                variant="ghost"
                size="sm"
                onclick={() => {
                  addingRule = false
                }}>{m.cancel()}</Button
              >
            </div>
          </div>
        {:else}
          <button
            type="button"
            class="mt-4 text-sm font-medium text-indigo-600 hover:underline dark:text-indigo-400"
            onclick={() => {
              addingRule = true
            }}
          >
            + {m.add_rule()}
          </button>
        {/if}
      </div>

      <!-- LOG TAB -->
    {:else if activeTab === 'log'}
      <div class="p-5">
        <!-- Filter -->
        <div class="mb-4 flex items-center gap-2">
          {#each ['', 'imported', 'failed', 'skipped', 'pending'] as status}
            <button
              type="button"
              class="rounded-full px-2.5 py-1 text-sm font-medium transition-colors
                {logFilter === status
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'}"
              onclick={() => {
                logFilter = status
              }}
            >
              {status === ''
                ? '全部'
                : status === 'imported'
                  ? '已导入'
                  : status === 'failed'
                    ? '失败'
                    : status === 'skipped'
                      ? '已跳过'
                      : '等待中'}
            </button>
          {/each}
          <button
            type="button"
            class="ml-auto text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
            onclick={() => ingressStore.loadLog(src.id, logFilter || undefined)}
            title="刷新"
          >
            <RefreshCw class="h-3.5 w-3.5" />
          </button>
        </div>

        {#if ingressStore.loadingLog}
          <GridSkeleton lines={5} />
        {:else if filteredLog.length === 0}
          <p class="text-md py-10 text-center text-gray-400">{m.empty_log()}</p>
        {:else}
          <div class="space-y-1.5">
            {#each filteredLog as entry (entry.id)}
              <div
                class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50"
              >
                <div class="flex items-start gap-2">
                  <div class="min-w-0 flex-1">
                    <p
                      class="truncate text-sm font-medium text-gray-800 dark:text-gray-200"
                    >
                      {entry.filename}
                    </p>
                    <div class="mt-0.5 flex items-center gap-2">
                      <span
                        class="inline-flex rounded-full px-1.5 py-0.5 text-xs font-medium {STATUS_STYLES[
                          entry.status
                        ] ?? ''}"
                      >
                        {STATUS_LABELS[entry.status] ?? entry.status}
                      </span>
                      <span class="text-xs text-gray-400"
                        >{formatDate(entry.imported_at)}</span
                      >
                    </div>
                    <Feedback class="mt-1" error={entry.error} />
                  </div>

                  <!-- Actions -->
                  <div class="flex shrink-0 items-center gap-1">
                    {#if entry.asset_id}
                      <a
                        href="/library?asset={entry.asset_id}"
                        class="text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400"
                        title={m.open_in_library()}
                      >
                        <ExternalLink class="h-3.5 w-3.5" />
                      </a>
                    {/if}
                    {#if entry.status === 'failed' || entry.status === 'skipped'}
                      <button
                        type="button"
                        class="text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400"
                        onclick={() => ingressStore.retryLogEntry(entry.id)}
                        title="重试"
                      >
                        <RotateCcw class="h-3.5 w-3.5" />
                      </button>
                    {/if}
                    <ButtonDelete
                      title={m.delete()}
                      onclick={() => ingressStore.deleteLogEntry(entry.id)}
                    />
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</aside>
