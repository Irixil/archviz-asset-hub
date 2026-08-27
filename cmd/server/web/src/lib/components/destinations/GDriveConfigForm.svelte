<script lang="ts">
  import { onMount } from 'svelte'
  import { integrationsApi, type OAuthConnection } from '$lib/api/client'

  interface GDriveConfig {
    connection_id: string
    workspace_id: string
    folder_id: string
    folder_name: string
  }

  interface Props {
    value: GDriveConfig
    errors?: Record<string, string>
  }

  let {
    value = $bindable({
      connection_id: '',
      workspace_id: '',
      folder_id: '',
      folder_name: '',
    }),
    errors = {},
  }: Props = $props()

  let connections = $state<OAuthConnection[]>([])
  let loading = $state(true)

  onMount(async () => {
    try {
      const all = await integrationsApi.list()
      connections = all.filter((c) => c.provider === 'google')
    } catch {
      // ignore
    } finally {
      loading = false
    }
  })

  const inputClass =
    'text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900'
</script>

<div class="space-y-3">
  {#if loading}
    <p class="text-sm text-gray-500 dark:text-gray-400">正在加载连接…</p>
  {:else if connections.length === 0}
    <div
      class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-800 dark:bg-gray-800/50"
    >
      <p class="mb-1 text-sm font-medium text-gray-900 dark:text-gray-100">
        尚未连接 Google 账号
      </p>
      <p class="text-sm text-gray-500 dark:text-gray-400">
        请先在
        <a
          href="/library/settings/integrations"
          class="text-indigo-600 underline hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300"
        >
          连接设置
        </a>
        中连接 Google 账号。
      </p>
    </div>
  {:else}
    <div>
      <label
        for="export-gdrive-account"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        Google 账号
      </label>
      <select
        id="export-gdrive-account"
        class={inputClass}
        bind:value={value.connection_id}
      >
        <option value="">选择账号</option>
        {#each connections as conn}
          <option value={conn.id}>{conn.provider_email ?? conn.id}</option>
        {/each}
      </select>
      {#if errors.connection_id}
        <p class="mt-1 text-xs text-red-500">{errors.connection_id}</p>
      {/if}
      {#if value.connection_id}
        {@const selectedConn = connections.find(
          (c) => c.id === value.connection_id
        )}
        {#if selectedConn && !selectedConn.scopes.includes('https://www.googleapis.com/auth/drive.file') && !selectedConn.scopes.includes('https://www.googleapis.com/auth/drive')}
          <p class="mt-1.5 text-sm text-amber-600 dark:text-amber-400">
            当前账号权限不足，请先<a
              href="/library/settings/integrations"
              class="underline">重新连接账号</a
            >
            后再使用导出。
          </p>
        {/if}
      {/if}
    </div>
  {/if}

  <div>
    <label
      for="export-gdrive-dest-id"
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      文件夹 ID
    </label>
    <input
      id="export-gdrive-dest-id"
      class="{inputClass} font-mono"
      bind:value={value.folder_id}
      placeholder="1BxiMVs0XRA5nFMdKJIb6bDqVZQ0_sWDj"
    />
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      从 Google Drive 地址中复制文件夹 ID： drive.google.com/drive/folders/<span
        class="font-medium">ID</span
      >
    </p>
    {#if errors.folder_id}
      <p class="mt-1 text-xs text-red-500">{errors.folder_id}</p>
    {/if}
  </div>

  <div>
    <label
      for="export-gdrive-folder-name"
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      文件夹名称 <span class="font-normal text-gray-400">（可选）</span>
    </label>
    <input
      id="export-gdrive-folder-name"
      class={inputClass}
      bind:value={value.folder_name}
      placeholder="建筑素材导出"
    />
  </div>
</div>
