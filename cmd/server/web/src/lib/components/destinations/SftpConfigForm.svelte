<script lang="ts">
  interface SftpConfig {
    host: string
    port: number
    username: string
    password: string
    private_key: string
    remote_path: string
  }

  interface Props {
    value: SftpConfig
    errors?: Record<string, string>
  }

  let {
    value = $bindable({
      host: '',
      port: 22,
      username: '',
      password: '',
      private_key: '',
      remote_path: '/',
    }),
    errors = {},
  }: Props = $props()

  let authTab = $state<'password' | 'key'>('password')

  const inputClass =
    'text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-900'
</script>

<div class="space-y-3">
  <div class="grid grid-cols-3 gap-3">
    <div class="col-span-2">
      <label
        for="sftp-host"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        主机地址
      </label>
      <input
        id="sftp-host"
        class={inputClass}
        bind:value={value.host}
        placeholder="files.example.com"
      />
      {#if errors.host}
        <p class="mt-1 text-xs text-red-500">{errors.host}</p>
      {/if}
    </div>
    <div>
      <label
        for="sftp-port"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        端口
      </label>
      <input
        id="sftp-port"
        class={inputClass}
        type="number"
        bind:value={value.port}
        min="1"
        max="65535"
      />
    </div>
  </div>

  <div>
    <label
      for="sftp-user"
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      用户名
    </label>
    <input
      id="sftp-user"
      class={inputClass}
      bind:value={value.username}
      placeholder="backup"
    />
    {#if errors.username}
      <p class="mt-1 text-xs text-red-500">{errors.username}</p>
    {/if}
  </div>

  <div>
    <label
      for="sftp-auth"
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      验证方式
    </label>
    <div
      class="mb-2 flex gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-gray-700 dark:bg-gray-800/50"
    >
      {#each [{ id: 'password', label: '密码' }, { id: 'key', label: '私钥' }] as tab}
        <button
          type="button"
          class="flex-1 rounded-md px-3 py-1 text-sm font-medium transition-colors {authTab ===
          tab.id
            ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-gray-100'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}"
          onclick={() => (authTab = tab.id as 'password' | 'key')}
        >
          {tab.label}
        </button>
      {/each}
    </div>
    {#if authTab === 'password'}
      <input
        class={inputClass}
        type="password"
        bind:value={value.password}
        placeholder="••••••••"
        autocomplete="new-password"
      />
      {#if errors.password}
        <p class="mt-1 text-xs text-red-500">{errors.password}</p>
      {/if}
    {:else}
      <textarea
        class="{inputClass} font-mono text-xs"
        rows={6}
        bind:value={value.private_key}
        placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
      ></textarea>
      {#if errors.private_key}
        <p class="mt-1 text-xs text-red-500">{errors.private_key}</p>
      {/if}
    {/if}
  </div>

  <div>
    <label
      for="sftp-path"
      class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
    >
      远程路径
    </label>
    <input
      id="sftp-path"
      class="{inputClass} font-mono"
      bind:value={value.remote_path}
      placeholder="/backups/damask"
    />
    {#if errors.remote_path}
      <p class="mt-1 text-xs text-red-500">{errors.remote_path}</p>
    {/if}
  </div>
</div>
