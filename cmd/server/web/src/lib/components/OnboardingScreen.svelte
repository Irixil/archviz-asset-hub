<script lang="ts">
  import { Upload, FolderOpen, Tag } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    onDismiss: () => void
    projectName?: string | null
  }

  let { onDismiss, projectName = null }: Props = $props()

  const steps = [
    {
      icon: Upload,
      title: m.onboard_upload(),
      body: m.onboard_upload_body(),
    },
    {
      icon: FolderOpen,
      title: m.onboard_project(),
      body: m.onboard_project_body(),
    },
    {
      icon: Tag,
      title: m.onboard_tags(),
      body: m.onboard_tags_body(),
    },
  ]

  function startUpload() {
    onDismiss()
    requestAnimationFrame(() => {
      document.querySelector<HTMLElement>('[data-upload-trigger]')?.click()
    })
  }
</script>

<div
  class="mx-auto flex max-w-4xl flex-col justify-center px-4 py-8 sm:min-h-[480px] sm:px-8"
>
  <div
    class="overflow-hidden rounded-lg border border-[var(--border-default)] bg-[var(--bg-surface)] xl:grid xl:grid-cols-[1.1fr_0.9fr]"
  >
    <div
      class="border-b border-[var(--border-subtle)] p-6 sm:p-8 xl:border-r xl:border-b-0"
    >
      <h1
        class="text-2xl font-bold tracking-tight text-[var(--text-primary)] sm:text-3xl"
      >
        {projectName ? `为“${projectName}”添加素材` : '建立第一个可用素材库'}
      </h1>
      <p class="mt-3 max-w-lg text-sm leading-6 text-[var(--text-secondary)]">
        {#if projectName}
          上传这个项目使用的模型、PSD、贴图与成果图，之后可继续用项目文件夹整理。
        {:else}
          先上传三维模型、PSD、贴图或参考图。系统会先按文件格式自动归类，再补充材质、空间和软件兼容信息。
        {/if}
      </p>
      <div class="mt-6 flex flex-wrap gap-3">
        <Button onclick={startUpload}
          >{projectName ? '向项目添加素材' : '上传第一批素材'}</Button
        >
        <button
          type="button"
          class="min-h-10 rounded-md px-3 text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]"
          onclick={onDismiss}>稍后浏览</button
        >
      </div>
      <p class="mt-5 text-xs text-[var(--text-muted)]">
        使用桌面版可把素材卡片直接拖到 3ds Max、Photoshop
        或文件夹；浏览器中仍可下载原文件。
      </p>
    </div>

    <ol class="divide-y divide-[var(--border-subtle)] bg-[var(--bg-stripe)]">
      {#each steps as step, index}
        {@const Icon = step.icon}
        <li class="flex gap-3 p-5 sm:p-6">
          <span
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-[var(--border-default)] bg-[var(--bg-surface)] text-[var(--accent)]"
            ><Icon class="h-4 w-4" /></span
          >
          <div>
            <p
              class="text-xs font-semibold tracking-wide text-[var(--text-muted)]"
            >
              0{index + 1}
            </p>
            <p class="mt-0.5 text-sm font-semibold text-[var(--text-primary)]">
              {step.title}
            </p>
            <p class="mt-1 text-xs leading-5 text-[var(--text-secondary)]">
              {step.body}
            </p>
          </div>
        </li>
      {/each}
    </ol>
  </div>
</div>
