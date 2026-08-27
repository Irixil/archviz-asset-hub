<script lang="ts">
  import { Eye, EyeOff } from '@lucide/svelte'
  import type { IngressSourceType } from '$lib/api'
  import { integrationsApi, type OAuthConnection } from '$lib/api'
  import Input from '$lib/components/ui/Input.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    type: IngressSourceType
    config: Record<string, unknown>
  }

  let { type, config = $bindable({}) }: Props = $props()

  // Helpers — two-way binding per field via get/set
  function field(key: string): string {
    return (config[key] as string) ?? ''
  }
  function setField(key: string, value: unknown) {
    config = { ...config, [key]: value }
  }
  function boolField(key: string, def = false): boolean {
    return key in config ? Boolean(config[key]) : def
  }

  // Password visibility toggles
  let showPassword = $state(false)
  let showSecretKey = $state(false)

  // OAuth connections for gdrive/canva
  let connections = $state<OAuthConnection[]>([])
  $effect(() => {
    if (type === 'gdrive' || type === 'canva') {
      integrationsApi
        .list()
        .then((c) => {
          connections = c
        })
        .catch(() => {})
    }
  })

  function connectionsFor(provider: string) {
    return connections.filter((c) => c.provider === provider)
  }
</script>

{#if type === 'email_api'}
  <!-- No config needed — handled by IV-5 UI -->
  <div
    class="text-md rounded-lg border border-indigo-100 bg-indigo-50/50 p-4 text-indigo-700 dark:border-indigo-900/50 dark:bg-indigo-900/20 dark:text-indigo-300"
  >
    无需额外配置。保存后，系统会生成一个专属素材收件邮箱。
  </div>
{:else if type === 'imap'}
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="IMAP 主机"
        placeholder="imap.gmail.com"
        value={field('host')}
        oninput={(e) => setField('host', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="端口"
        type="number"
        placeholder="993"
        value={field('port') || '993'}
        oninput={(e) =>
          setField('port', Number((e.target as HTMLInputElement).value))}
      />
    </div>
    <div class="flex items-center gap-2">
      <input
        id="imap-tls"
        type="checkbox"
        checked={boolField('tls', true)}
        onchange={(e) =>
          setField('tls', (e.target as HTMLInputElement).checked)}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label for="imap-tls" class="text-md text-gray-700 dark:text-gray-300"
        >使用 TLS</label
      >
    </div>
    <Input
      label="用户名"
      placeholder="you@gmail.com"
      value={field('username')}
      oninput={(e) =>
        setField('username', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="密码 / 应用专用密码"
        type={showPassword ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('password')}
        oninput={(e) =>
          setField('password', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute top-8 right-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => {
          showPassword = !showPassword
        }}
        tabindex="-1"
      >
        {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye
            class="h-4 w-4"
          />{/if}
      </button>
    </div>
    <Input
      label="邮箱文件夹"
      placeholder="INBOX"
      value={field('mailbox') || 'INBOX'}
      oninput={(e) => setField('mailbox', (e.target as HTMLInputElement).value)}
    />
    <div>
      <label
        for="imap-after-import"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >导入后处理</label
      >
      <select
        id="imap-after-import"
        value={field('after_import') || 'mark_read'}
        onchange={(e) =>
          setField('after_import', (e.target as HTMLSelectElement).value)}
        class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="mark_read">标记为已读</option>
        <option value="move_to">移至其他邮箱文件夹</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="移至邮箱文件夹"
        placeholder="Imported"
        value={field('move_to_mailbox')}
        oninput={(e) =>
          setField('move_to_mailbox', (e.target as HTMLInputElement).value)}
      />
    {/if}
  </div>
{:else if type === 'sftp'}
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="主机地址"
        placeholder="files.client.com"
        value={field('host')}
        oninput={(e) => setField('host', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="端口"
        type="number"
        placeholder="22"
        value={field('port') || '22'}
        oninput={(e) =>
          setField('port', Number((e.target as HTMLInputElement).value))}
      />
    </div>
    <Input
      label="用户名"
      placeholder="uploader"
      value={field('username')}
      oninput={(e) =>
        setField('username', (e.target as HTMLInputElement).value)}
    />
    <div>
      <label
        for="sftp-auth-method"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >验证方式</label
      >
      <select
        id="sftp-auth-method"
        value={field('auth_method') || 'password'}
        onchange={(e) =>
          setField('auth_method', (e.target as HTMLSelectElement).value)}
        class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="password">密码</option>
        <option value="key">私钥</option>
      </select>
    </div>
    {#if field('auth_method') !== 'key'}
      <div class="relative">
        <Input
          label="密码"
          type={showPassword ? 'text' : 'password'}
          placeholder="••••••••"
          value={field('password')}
          oninput={(e) =>
            setField('password', (e.target as HTMLInputElement).value)}
        />
        <button
          type="button"
          class="absolute top-8 right-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
          onclick={() => {
            showPassword = !showPassword
          }}
          tabindex="-1"
        >
          {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye
              class="h-4 w-4"
            />{/if}
        </button>
      </div>
    {:else}
      <div class="relative">
        <label
          for="sftp-private-key"
          class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
          >私钥（PEM）</label
        >
        <textarea
          id="sftp-private-key"
          rows="4"
          placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;…"
          value={field('private_key')}
          oninput={(e) =>
            setField('private_key', (e.target as HTMLTextAreaElement).value)}
          class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 font-mono text-sm text-gray-900 shadow-sm focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        ></textarea>
      </div>
    {/if}
    <Input
      label="远程路径"
      placeholder="/company-assets/项目资料"
      value={field('remote_path')}
      oninput={(e) =>
        setField('remote_path', (e.target as HTMLInputElement).value)}
    />
    <Input
      label="服务器指纹（推荐）"
      placeholder="SHA256:…"
      value={field('host_key_fingerprint')}
      oninput={(e) =>
        setField('host_key_fingerprint', (e.target as HTMLInputElement).value)}
    />
    <div
      class="space-y-3 rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-gray-800 dark:bg-gray-800/50"
    >
      <label
        class="text-md flex items-start gap-2 text-gray-700 dark:text-gray-300"
      >
        <input
          type="checkbox"
          checked={boolField('recursive', true)}
          onchange={(e) =>
            setField('recursive', (e.target as HTMLInputElement).checked)}
          class="mt-0.5 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
        />
        <span>扫描全部子目录，批量导入新增素材</span>
      </label>
      <label
        class="text-md flex items-start gap-2 text-gray-700 dark:text-gray-300"
      >
        <input
          type="checkbox"
          checked={boolField('insecure_host_key', false)}
          onchange={(e) =>
            setField(
              'insecure_host_key',
              (e.target as HTMLInputElement).checked
            )}
          class="mt-0.5 h-4 w-4 rounded border-gray-300 text-amber-600 focus:ring-amber-500"
        />
        <span>暂时跳过服务器指纹校验，仅限可信内网</span>
      </label>
      <p class="text-sm text-gray-400 dark:text-gray-500">
        导入不会移动或删除服务器上的原文件。
      </p>
    </div>
  </div>
{:else if type === 'dav'}
  <div class="space-y-4">
    <Input
      label="WebDAV URL"
      placeholder="https://cloud.example.com/remote.php/dav/files/user/Uploads/"
      value={field('url')}
      oninput={(e) => setField('url', (e.target as HTMLInputElement).value)}
    />
    <p class="text-sm text-gray-400 dark:text-gray-500">
      Nextcloud: <code class="rounded bg-gray-100 px-1 dark:bg-gray-800"
        >{'{nextcloud_url}'}/remote.php/dav/files/{'{username}'}/</code
      >
    </p>
    <Input
      label="用户名"
      placeholder="user"
      value={field('username')}
      oninput={(e) =>
        setField('username', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="密码 / 应用专用密码"
        type={showPassword ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('password')}
        oninput={(e) =>
          setField('password', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute top-8 right-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => {
          showPassword = !showPassword
        }}
        tabindex="-1"
      >
        {#if showPassword}<EyeOff class="h-4 w-4" />{:else}<Eye
            class="h-4 w-4"
          />{/if}
      </button>
    </div>
    <div>
      <label
        for="dav-after-import"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >导入后处理</label
      >
      <select
        id="dav-after-import"
        value={field('after_import') || 'leave'}
        onchange={(e) =>
          setField('after_import', (e.target as HTMLSelectElement).value)}
        class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="leave">保留在原位置</option>
        <option value="move_to">移至其他地址</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="移至 WebDAV 地址"
        placeholder="https://cloud.example.com/remote.php/dav/files/user/Done/"
        value={field('move_to_url')}
        oninput={(e) =>
          setField('move_to_url', (e.target as HTMLInputElement).value)}
      />
    {/if}
  </div>
{:else if type === 's3'}
  <div class="space-y-4">
    <Input
      label="服务地址（使用 AWS 时可留空）"
      placeholder="https://s3.amazonaws.com"
      value={field('endpoint')}
      oninput={(e) =>
        setField('endpoint', (e.target as HTMLInputElement).value)}
    />
    <div class="grid grid-cols-2 gap-4">
      <Input
        label="区域"
        placeholder="us-east-1"
        value={field('region')}
        oninput={(e) =>
          setField('region', (e.target as HTMLInputElement).value)}
      />
      <Input
        label="存储桶"
        placeholder="client-uploads"
        value={field('bucket')}
        oninput={(e) =>
          setField('bucket', (e.target as HTMLInputElement).value)}
      />
    </div>
    <Input
      label="路径前缀（可选）"
      placeholder="incoming/"
      value={field('prefix')}
      oninput={(e) => setField('prefix', (e.target as HTMLInputElement).value)}
    />
    <Input
      label="访问密钥 ID"
      placeholder="AKIA…"
      value={field('access_key_id')}
      oninput={(e) =>
        setField('access_key_id', (e.target as HTMLInputElement).value)}
    />
    <div class="relative">
      <Input
        label="访问密钥"
        type={showSecretKey ? 'text' : 'password'}
        placeholder="••••••••"
        value={field('secret_access_key')}
        oninput={(e) =>
          setField('secret_access_key', (e.target as HTMLInputElement).value)}
      />
      <button
        type="button"
        class="absolute top-8 right-3 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        onclick={() => {
          showSecretKey = !showSecretKey
        }}
        tabindex="-1"
      >
        {#if showSecretKey}<EyeOff class="h-4 w-4" />{:else}<Eye
            class="h-4 w-4"
          />{/if}
      </button>
    </div>
    <div>
      <label
        for="s3-after-import"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
        >导入后处理</label
      >
      <select
        id="s3-after-import"
        value={field('after_import') || 'leave'}
        onchange={(e) =>
          setField('after_import', (e.target as HTMLSelectElement).value)}
        class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="leave">保留在原位置</option>
        <option value="move_to">移至其他路径前缀</option>
        <option value="delete">{m.delete()}</option>
      </select>
    </div>
    {#if field('after_import') === 'move_to'}
      <Input
        label="移至路径前缀"
        placeholder="done/"
        value={field('move_to_prefix')}
        oninput={(e) =>
          setField('move_to_prefix', (e.target as HTMLInputElement).value)}
      />
    {/if}
    <div class="flex items-center gap-2">
      <input
        id="s3-path-style"
        type="checkbox"
        checked={boolField('use_path_style', false)}
        onchange={(e) =>
          setField('use_path_style', (e.target as HTMLInputElement).checked)}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label
        for="s3-path-style"
        class="text-md text-gray-700 dark:text-gray-300"
      >
        使用路径式地址（MinIO 必需）
      </label>
    </div>
  </div>
{:else if type === 'gdrive'}
  {@const googleConns = connectionsFor('google')}
  <div class="space-y-4">
    <div>
      <label
        for="gdrive-connection"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        {m.ingress_gdrive_connection()}
      </label>
      {#if googleConns.length === 0}
        <p class="text-sm text-zinc-500 dark:text-zinc-400">
          {m.ingress_gdrive_connection_empty()}
          <a
            href="/library/settings/integrations"
            class="text-blue-600 hover:underline dark:text-blue-400"
          >
            {m.ingress_gdrive_connect_link()}
          </a>
        </p>
      {:else}
        <select
          id="gdrive-connection"
          value={field('connection_id')}
          onchange={(e) =>
            setField('connection_id', (e.target as HTMLSelectElement).value)}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        >
          <option value="">—</option>
          {#each googleConns as conn (conn.id)}
            <option value={conn.id}>{conn.provider_email ?? conn.id}</option>
          {/each}
        </select>
      {/if}
    </div>
    <div>
      <Input
        label={m.ingress_gdrive_folder_id()}
        placeholder="1BxiMVs0XRA5nFMd…"
        value={field('folder_id')}
        oninput={(e) =>
          setField('folder_id', (e.target as HTMLInputElement).value)}
      />
      <p class="mt-1 text-sm text-zinc-400 dark:text-zinc-500">
        {m.ingress_gdrive_folder_id_hint()}
      </p>
    </div>
    <div class="flex items-center gap-2">
      <input
        id="gdrive-subfolders"
        type="checkbox"
        checked={boolField('include_subfolders', false)}
        onchange={(e) =>
          setField(
            'include_subfolders',
            (e.target as HTMLInputElement).checked
          )}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label
        for="gdrive-subfolders"
        class="text-md text-gray-700 dark:text-gray-300"
      >
        {m.ingress_gdrive_include_subfolders()}
      </label>
    </div>
    <div class="flex items-center gap-2">
      <input
        id="gdrive-export-docs"
        type="checkbox"
        checked={boolField('export_google_docs', true)}
        onchange={(e) =>
          setField(
            'export_google_docs',
            (e.target as HTMLInputElement).checked
          )}
        class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
      />
      <label
        for="gdrive-export-docs"
        class="text-md text-gray-700 dark:text-gray-300"
      >
        {m.ingress_gdrive_export_google_docs()}
      </label>
    </div>
    <div>
      <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
        After import
      </p>
      <div class="space-y-2">
        <label
          class="text-md flex items-center gap-2 text-gray-700 dark:text-gray-300"
        >
          <input
            type="radio"
            name="gdrive-after"
            value="leave"
            checked={field('after_import') !== 'move_to_trash'}
            onchange={() => setField('after_import', 'leave')}
            class="text-indigo-600 focus:ring-indigo-500"
          />
          {m.ingress_gdrive_after_leave()}
        </label>
        <label
          class="text-md flex items-center gap-2 text-gray-700 dark:text-gray-300"
        >
          <input
            type="radio"
            name="gdrive-after"
            value="move_to_trash"
            checked={field('after_import') === 'move_to_trash'}
            onchange={() => setField('after_import', 'move_to_trash')}
            class="text-indigo-600 focus:ring-indigo-500"
          />
          {m.ingress_gdrive_after_trash()}
        </label>
      </div>
    </div>
  </div>
{:else if type === 'canva'}
  {@const canvaConns = connectionsFor('canva')}
  <div class="space-y-4">
    <div>
      <label
        for="canva-connection"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        {m.ingress_canva_connection()}
      </label>
      {#if canvaConns.length === 0}
        <p class="text-sm text-zinc-500 dark:text-zinc-400">
          {m.ingress_canva_connection_empty()}
          <a
            href="/library/settings/integrations"
            class="text-blue-600 hover:underline dark:text-blue-400"
          >
            {m.ingress_canva_connect_link()}
          </a>
        </p>
      {:else}
        <select
          id="canva-connection"
          value={field('connection_id')}
          onchange={(e) =>
            setField('connection_id', (e.target as HTMLSelectElement).value)}
          class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        >
          <option value="">—</option>
          {#each canvaConns as conn (conn.id)}
            <option value={conn.id}>{conn.provider_email ?? conn.id}</option>
          {/each}
        </select>
      {/if}
    </div>
    <div>
      <label
        for="canva-format"
        class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
      >
        {m.ingress_canva_export_format()}
      </label>
      <select
        id="canva-format"
        value={field('export_format') || 'pdf'}
        onchange={(e) =>
          setField('export_format', (e.target as HTMLSelectElement).value)}
        class="text-md w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      >
        <option value="pdf">{m.ingress_canva_format_pdf()}</option>
        <option value="png">{m.ingress_canva_format_png()}</option>
        <option value="jpg">{m.ingress_canva_format_jpg()}</option>
      </select>
    </div>
    <div>
      <p class="text-md mb-2 font-medium text-gray-700 dark:text-gray-300">
        {m.ingress_canva_ownership()}
      </p>
      <div class="space-y-2">
        {#each [{ value: 'owned', label: m.ingress_canva_ownership_owned() }, { value: 'shared', label: m.ingress_canva_ownership_shared() }, { value: 'any', label: m.ingress_canva_ownership_any() }] as opt (opt.value)}
          <label
            class="text-md flex items-center gap-2 text-gray-700 dark:text-gray-300"
          >
            <input
              type="radio"
              name="canva-ownership"
              value={opt.value}
              checked={(field('ownership') || 'owned') === opt.value}
              onchange={() => setField('ownership', opt.value)}
              class="text-indigo-600 focus:ring-indigo-500"
            />
            {opt.label}
          </label>
        {/each}
      </div>
    </div>
    <div>
      <Input
        label={m.ingress_canva_name_filter()}
        placeholder={m.ingress_canva_name_filter_hint()}
        value={field('name_filter')}
        oninput={(e) =>
          setField('name_filter', (e.target as HTMLInputElement).value)}
      />
    </div>
  </div>
{/if}
