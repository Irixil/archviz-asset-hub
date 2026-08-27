<script lang="ts">
  import {
    Mail,
    Server,
    HardDrive,
    Cloud,
    Inbox,
    ChevronLeft,
    FolderOpen,
    Palette,
  } from '@lucide/svelte'
  import type { IngressSource, IngressSourceType } from '$lib/api'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { ingressApi } from '$lib/api'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import SourceConfigForm from './SourceConfigForm.svelte'
  import Hint from '../ui/Hint.svelte'
  import Feedback from '../ui/Feedback.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    open?: boolean
    onadded: (source: IngressSource) => void
    onclose: () => void
  }

  let { open = $bindable(false), onadded, onclose }: Props = $props()

  type Step = 'pick' | 'configure'

  const SOURCE_TYPES: {
    type: IngressSourceType
    label: string
    desc: string
    icon: typeof Mail
  }[] = [
    {
      type: 'sftp',
      label: m.ingress_sftp(),
      desc: m.ingress_sftp_desc(),
      icon: Server,
    },
    {
      type: 'email_api',
      label: m.ingress_own_mail(),
      desc: m.ingress_own_mail_desc(),
      icon: Mail,
    },
    {
      type: 'imap',
      label: m.ingress_imap(),
      desc: m.ingress_imap_desc(),
      icon: Inbox,
    },
    {
      type: 'dav',
      label: m.ingress_dav(),
      desc: m.ingress_dav_desc(),
      icon: HardDrive,
    },
    {
      type: 's3',
      label: m.ingress_s3(),
      desc: m.ingress_s3_desc(),
      icon: Cloud,
    },
    {
      type: 'gdrive',
      label: m.ingress_gdrive_label(),
      desc: m.ingress_gdrive_folder_id_hint(),
      icon: FolderOpen,
    },
    {
      type: 'canva',
      label: m.ingress_canva_label(),
      desc: m.ingress_canva_export_format(),
      icon: Palette,
    },
  ]

  const POLL_INTERVALS = [
    { label: '5 分钟', value: 5 },
    { label: '15 分钟', value: 15 },
    { label: '30 分钟', value: 30 },
    { label: '1 小时', value: 60 },
    { label: '6 小时', value: 360 },
  ]

  let step = $state<Step>('pick')
  let selectedType = $state<IngressSourceType | null>(null)

  // Common fields
  let label = $state('')
  let destProjectId = $state('')
  let pollIntervalMin = $state(15)

  // Source-specific config (managed by SourceConfigForm)
  let sourceConfig = $state<Record<string, unknown>>({})

  // Test connection state
  let testStatus = $state<'idle' | 'testing' | 'ok' | 'error'>('idle')
  let testError = $state('')
  let createdSourceId = $state<string | null>(null)

  // Submission
  let saving = $state(false)
  let errors = $state<Record<string, string>>({})

  function pickType(type: IngressSourceType) {
    selectedType = type
    step = 'configure'
    label = ''
    destProjectId = projectsStore.projects[0]?.id ?? ''
    pollIntervalMin = 15
    sourceConfig =
      type === 'sftp'
        ? { port: 22, auth_method: 'password', recursive: true }
        : {}
    testStatus = 'idle'
    testError = ''
    createdSourceId = null
    errors = {}
  }

  function back() {
    step = 'pick'
    selectedType = null
    createdSourceId = null
  }

  function validate(): boolean {
    const e: Record<string, string> = {}
    if (!label.trim()) e.label = m.label_required()
    if (!destProjectId) e.destProjectId = m.dest_project_required()
    if (selectedType === 'sftp') {
      const text = (key: string) => String(sourceConfig[key] ?? '').trim()
      if (!text('host') || !text('username') || !text('remote_path')) {
        e.config = '请填写主机地址、用户名和远程路径。'
      } else if (
        (text('auth_method') || 'password') === 'key'
          ? !text('private_key')
          : !text('password')
      ) {
        e.config = '请填写服务器登录凭据。'
      } else if (
        !text('host_key_fingerprint') &&
        !Boolean(sourceConfig.insecure_host_key)
      ) {
        e.config = '请填写服务器指纹，或明确选择暂时跳过指纹校验。'
      }
    }
    errors = e
    return Object.keys(e).length === 0
  }

  // Create (or find already-created) source to enable "Test connection"
  async function ensureSourceCreated(): Promise<string | null> {
    if (createdSourceId) return createdSourceId
    if (!validate()) return null

    saving = true
    try {
      const src = await ingressStore.createSource({
        type: selectedType!,
        label: label.trim(),
        config: sourceConfig,
        dest_project_id: destProjectId || undefined,
        poll_interval_min: pollIntervalMin,
        enabled: false, // not enabled until user finishes
      })
      if (!src) return null
      createdSourceId = src.id
      return src.id
    } finally {
      saving = false
    }
  }

  async function testConnection() {
    const id = await ensureSourceCreated()
    if (!id) return
    testStatus = 'testing'
    testError = ''
    try {
      await ingressApi.test(id)
      testStatus = 'ok'
    } catch (e: unknown) {
      testStatus = 'error'
      testError = e instanceof Error ? e.message : m.test_connection_failed()
    }
  }

  async function handleSave() {
    if (!validate()) return
    saving = true
    try {
      let src: IngressSource | null
      if (createdSourceId) {
        // Update the draft with final values + enable it
        src = await ingressStore.updateSource(createdSourceId, {
          label: label.trim(),
          config: sourceConfig,
          dest_project_id: destProjectId || undefined,
          poll_interval_min: pollIntervalMin,
          enabled: true,
        })
      } else {
        src = await ingressStore.createSource({
          type: selectedType!,
          label: label.trim(),
          config: sourceConfig,
          dest_project_id: destProjectId || undefined,
          poll_interval_min: pollIntervalMin,
          enabled: true,
        })
      }
      if (src) {
        open = false
        onadded(src)
      }
    } finally {
      saving = false
    }
  }

  const selectedTypeMeta = $derived(
    SOURCE_TYPES.find((t) => t.type === selectedType)
  )
</script>

<Modal bind:open {onclose}>
  {#if step === 'pick'}
    <!-- Step 1: Type picker -->
    <div class="px-6 py-5">
      <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-50">
        {m.add_ingress_source()}
      </h2>
      <Hint>{m.add_ingress_subtitle()}</Hint>
    </div>

    <div class="grid grid-cols-1 gap-3 px-6 pb-6 sm:grid-cols-2">
      {#each SOURCE_TYPES as { type, label: typeLabel, desc, icon: Icon }}
        <button
          type="button"
          class="flex items-start gap-3 rounded-xl border border-gray-100 bg-white p-4 text-left transition-colors
            hover:border-indigo-300 hover:bg-indigo-50/40
            dark:border-gray-800 dark:bg-gray-900 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20"
          onclick={() => pickType(type)}
        >
          <div
            class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-500 dark:bg-indigo-900/30 dark:text-indigo-400"
          >
            <Icon class="h-5 w-5" />
          </div>
          <div>
            <Hint class="text-md font-semibold">{typeLabel}</Hint>
            <Hint class="text-sm">{desc}</Hint>
          </div>
        </button>
      {/each}
    </div>
  {:else}
    <!-- Step 2: Configure -->
    <div
      class="flex items-center gap-3 border-b border-gray-100 px-6 py-4 dark:border-gray-800"
    >
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        onclick={back}
      >
        <ChevronLeft class="h-4 w-4" />
      </button>
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-50">
          {selectedTypeMeta?.label}
        </h2>
        <Hint class="text-sm">{selectedTypeMeta?.desc}</Hint>
      </div>
    </div>

    <div class="space-y-5 px-6 py-5">
      <!-- Common fields -->
      <Input
        id="source-label"
        label="名称"
        placeholder={selectedType === 'sftp'
          ? '设计部共享素材'
          : m.example_client_uploads()}
        bind:value={label}
        error={errors.label}
        required
      />

      <div class="grid grid-cols-2 gap-4">
        <!-- Destination project -->
        <div>
          <label
            for="dest-project"
            class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
          >
            {m.dest_project()}
          </label>
          <select
            id="dest-project"
            bind:value={destProjectId}
            class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
          >
            <option value="">{m.none_choice()}</option>
            {#each projectsStore.projects as p (p.id)}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
          <Feedback error={errors.destProjectId} />
        </div>

        <!-- Poll interval (hidden for email_api) -->
        {#if selectedType !== 'email_api'}
          <div>
            <label
              for="poll-interval"
              class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
            >
              {m.poll_interval()}
            </label>
            <select
              id="poll-interval"
              bind:value={pollIntervalMin}
              class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
            >
              {#each POLL_INTERVALS as opt (opt.value)}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>

      <!-- Source-specific config form -->
      <SourceConfigForm type={selectedType!} bind:config={sourceConfig} />
      <Feedback error={errors.config} />

      {#if selectedType === 'sftp'}
        <Hint
          class="rounded-lg bg-indigo-50 px-3 py-2 text-sm text-indigo-700 dark:bg-indigo-900/20 dark:text-indigo-300"
        >
          当前服务器目录中的所有新增文件会批量归入所选项目；同一文件不会重复导入。
        </Hint>
      {/if}

      <!-- Test connection -->
      {#if selectedType !== 'email_api'}
        <div
          class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-800 dark:bg-gray-800/50"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="text-md text-gray-600 dark:text-gray-300">
              {#if testStatus === 'idle'}
                {m.test_connection_before()}
              {:else if testStatus === 'testing'}
                {m.testing_connection()}
              {:else if testStatus === 'ok'}
                <Feedback class="mt-0 bg-transparent" success="连接成功" />
              {:else}
                <Feedback class="mt-0 bg-transparent" error={testError} />
              {/if}
            </div>
            <Button
              variant="secondary"
              size="sm"
              loading={testStatus === 'testing'}
              onclick={testConnection}
            >
              {m.test_connection()}
            </Button>
          </div>
        </div>
      {/if}
    </div>

    <!-- Footer actions -->
    <div
      class="flex items-center justify-end gap-2 border-t border-gray-100 px-6 py-4 dark:border-gray-800"
    >
      <Button variant="secondary" onclick={onclose}>{m.cancel()}</Button>
      <Button variant="primary" loading={saving} onclick={handleSave}>
        {m.save()}
      </Button>
    </div>
  {/if}
</Modal>
