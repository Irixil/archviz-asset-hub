<script lang="ts">
  import { Server, FolderOpen } from '@lucide/svelte'

  interface Props {
    value: 'sftp' | 'gdrive'
    onchange?: (v: 'sftp' | 'gdrive') => void
  }

  let { value = $bindable('sftp'), onchange }: Props = $props()

  function pick(v: 'sftp' | 'gdrive') {
    value = v
    onchange?.(v)
  }

  const options = [
    {
      id: 'sftp' as const,
      label: 'SFTP 服务器',
      desc: '上传到 SSH/SFTP 服务器',
      Icon: Server,
    },
    {
      id: 'gdrive' as const,
      label: 'Google Drive',
      desc: '上传到 Google Drive 文件夹',
      Icon: FolderOpen,
    },
  ]
</script>

<div class="grid grid-cols-2 gap-3">
  {#each options as opt}
    <button
      type="button"
      class="flex items-start gap-3 rounded-lg border p-3 text-left transition-colors {value ===
      opt.id
        ? 'border-indigo-300 bg-indigo-50/40 dark:border-indigo-700 dark:bg-indigo-900/20'
        : 'border-gray-100 hover:border-gray-200 dark:border-gray-800 dark:hover:border-gray-700'}"
      onclick={() => pick(opt.id)}
    >
      <opt.Icon
        class="mt-0.5 size-5 shrink-0 text-gray-400 dark:text-gray-500"
      />
      <div>
        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
          {opt.label}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400">{opt.desc}</p>
      </div>
    </button>
  {/each}
</div>
