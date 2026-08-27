<script lang="ts">
  import { onMount } from 'svelte'
  import type {
    WorkflowGraphNode,
    WorkflowNodeConfigSchema,
    WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import { folderApi, tagApi, type Folder } from '$lib/api'
  import type { Tag } from '$lib/api'
  import Input from '$lib/components/ui/Input.svelte'
  import AIVisionModelSelect from '$lib/components/AIVisionModelSelect.svelte'
  import AILanguageSelect from '$lib/components/AILanguageSelect.svelte'
  import OCRLanguageSelect from '$lib/components/OCRLanguageSelect.svelte'
  import AIDescriptionPromptInput from '$lib/components/AIDescriptionPromptInput.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { variantTypes, variantTypeMap } from '$lib/stores/variantTypes.svelte'

  interface Props {
    node: WorkflowGraphNode | null
    schema: WorkflowNodeSchema | null
    onUpdate?: (config: Record<string, unknown>) => void
    onDelete?: () => void
    onDuplicate?: () => void
    readonly?: boolean
  }

  let {
    node,
    schema,
    onUpdate = () => {},
    onDelete = () => {},
    onDuplicate = () => {},
    readonly = false,
  }: Props = $props()

  const configSchema = $derived(schema?.config_schema ?? null)
  const properties = $derived(
    Object.entries(configSchema?.properties ?? {}) as Array<
      [string, WorkflowNodeConfigSchema]
    >
  )

  function currentConfig() {
    return node?.config ?? {}
  }

  function updateField(key: string, value: unknown) {
    onUpdate({ ...currentConfig(), [key]: value })
  }

  function updateJsonField(key: string, raw: string) {
    if (raw.trim() === '') {
      const next = { ...currentConfig() }
      delete next[key]
      onUpdate(next)
      return
    }
    try {
      updateField(key, JSON.parse(raw))
    } catch {
      updateField(key, raw)
    }
  }

  function stringValue(key: string) {
    const value = currentConfig()[key]
    if (typeof value === 'string') return value
    if (value == null) return ''
    return String(value)
  }

  function jsonValue(key: string) {
    const value = currentConfig()[key]
    if (value == null || value === '') return ''
    if (typeof value === 'string') return value
    return JSON.stringify(value, null, 2)
  }

  function numberValue(key: string) {
    const value = currentConfig()[key]
    if (typeof value === 'number') return value
    if (typeof value === 'string' && value !== '') return Number(value)
    return undefined
  }

  function boolValue(key: string) {
    return Boolean(currentConfig()[key])
  }

  function arrayValue(key: string) {
    const value = currentConfig()[key]
    if (!Array.isArray(value)) return ''
    return value.join('\n')
  }

  // Project / folder selectors
  let folders = $state<readonly Folder[]>([])
  let foldersLoading = $state(false)

  // Tag selector
  let tags = $state<readonly Tag[]>([])

  onMount(() => {
    if (projectsStore.projects.length === 0) projectsStore.load()
    tagApi
      .list()
      .then((data) => {
        tags = data
      })
      .catch(() => {})
  })

  const selectedProjectId = $derived(
    (currentConfig()['project_id'] as string | undefined) ?? ''
  )

  $effect(() => {
    if (!selectedProjectId) {
      folders = []
      return
    }
    foldersLoading = true
    folderApi
      .list(selectedProjectId)
      .then((data) => {
        folders = data
      })
      .catch(() => {
        folders = []
      })
      .finally(() => {
        foldersLoading = false
      })
  })

  function flattenFolders(
    list: readonly Folder[],
    depth = 0
  ): { folder: Folder; depth: number }[] {
    return list.flatMap((f) => [
      { folder: f, depth },
      ...flattenFolders(f.children ?? [], depth + 1),
    ])
  }

  const flatFolders = $derived(flattenFolders(folders))
</script>

<div
  class="flex flex-col rounded-tl-[24px] rounded-bl-[24px] border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
>
  <div
    class="flex items-start justify-between gap-3 border-b border-[var(--border-subtle)] px-4 py-3.5"
  >
    <div class="min-w-0">
      <p
        class="text-[10px] font-semibold tracking-[0.18em] text-[var(--text-muted)] uppercase"
      >
        Node Config
      </p>
      <h3 class="mt-1 text-sm font-semibold text-[var(--text-primary)]">
        {schema?.label ?? 'No selection'}
      </h3>
      {#if schema?.description}
        <p class="mt-0.5 text-xs leading-relaxed text-[var(--text-secondary)]">
          {schema.description}
        </p>
      {/if}
    </div>

    {#if node && !readonly}
      <div class="flex shrink-0 gap-1.5">
        <button
          type="button"
          class="rounded-lg border border-[var(--border-default)] px-2.5 py-1 text-[11px] font-semibold text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-elevated)]"
          onclick={() => onDuplicate()}
        >
          Duplicate
        </button>
        <button
          type="button"
          class="rounded-lg border border-rose-500/25 px-2.5 py-1 text-[11px] font-semibold text-rose-600 transition-colors hover:bg-rose-500/8 dark:text-rose-400"
          onclick={() => onDelete()}
        >
          Delete
        </button>
      </div>
    {/if}
  </div>

  <div class="flex-1 p-4">
    {#if !node || !schema}
      <p class="text-xs leading-relaxed text-[var(--text-muted)]">
        Select a node on the canvas to configure it.
      </p>
    {:else if properties.length === 0}
      <p class="text-xs text-[var(--text-muted)]">
        No configuration options for this node.
      </p>
    {:else}
      <div class="space-y-4">
        {#each properties as [key, field] (key)}
          <div class="space-y-1.5">
            {#if key === 'project_id'}
              <label
                for="project-id-select"
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? 'Project'}
              </label>
              <select
                id="project-id-select"
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
                disabled={readonly}
                value={stringValue(key)}
                onchange={(event) => {
                  const val = (event.currentTarget as HTMLSelectElement).value
                  const next: Record<string, unknown> = {
                    ...currentConfig(),
                    project_id: val,
                  }
                  delete next['folder_id']
                  onUpdate(next)
                }}
              >
                <option value="">Select project…</option>
                {#each projectsStore.projects as project (project.id)}
                  <option value={project.id}>{project.name}</option>
                {/each}
              </select>
            {:else if key === 'folder_id'}
              <label
                for="folder-id-select"
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? 'Folder'}
              </label>
              <select
                id="folder-id-select"
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
                disabled={readonly || !selectedProjectId || foldersLoading}
                value={stringValue(key)}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLSelectElement).value
                  )}
              >
                <option value="">
                  {#if !selectedProjectId}
                    Select a project first
                  {:else if foldersLoading}
                    Loading…
                  {:else}
                    Root (no folder)
                  {/if}
                </option>
                {#each flatFolders as { folder, depth } (folder.id)}
                  <option value={folder.id}>
                    {'  '.repeat(depth)}{folder.name}
                  </option>
                {/each}
              </select>
            {:else if field.format === 'variant'}
              <label
                for={'variant-type-' + key}
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <select
                id={'variant-type-' + key}
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
                disabled={readonly}
                value={stringValue(key)}
                onchange={(event) => {
                  const val = (event.currentTarget as HTMLSelectElement).value
                  updateField(key, val)
                  const def = variantTypeMap.get(val)
                  if (def && !currentConfig()['params']) {
                    updateField('params', def.paramsExample)
                  }
                }}
              >
                <option value="">Select variant type…</option>
                {#each ['image', 'video', 'audio'] as category (category)}
                  <optgroup
                    label={category.charAt(0).toUpperCase() + category.slice(1)}
                  >
                    {#each variantTypes.filter((v) => v.category === category) as vt (vt.value)}
                      <option value={vt.value}>{vt.label}</option>
                    {/each}
                  </optgroup>
                {/each}
              </select>
            {:else if field.format === 'vision_model'}
              <AIVisionModelSelect
                value={stringValue(key)}
                disabled={readonly}
                onchange={(val) => updateField(key, val)}
              />
            {:else if field.format === 'language'}
              <AILanguageSelect
                value={stringValue(key)}
                disabled={readonly}
                onchange={(val) => updateField(key, val)}
              />
            {:else if field.format === 'ocr_language'}
              <OCRLanguageSelect
                value={stringValue(key)}
                disabled={readonly}
                onchange={(val) => updateField(key, val)}
              />
            {:else if field.format === 'ai_description_prompt'}
              <AIDescriptionPromptInput
                value={stringValue(key)}
                disabled={readonly}
                onchange={(val) => updateField(key, val)}
              />
            {:else if field.format === 'tag'}
              <label
                for={'tag-value-' + key}
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <select
                id={'tag-value-' + key}
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
                disabled={readonly}
                value={stringValue(key)}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLSelectElement).value
                  )}
              >
                <option value="">Select tag…</option>
                {#each tags as tag (tag.name)}
                  <option value={tag.name}>{tag.name}</option>
                {/each}
              </select>
            {:else if field.enum && field.enum.length > 0}
              <label
                for={'enum-value-' + key}
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <select
                id={'enum-value-' + key}
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none"
                disabled={readonly}
                value={stringValue(key)}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLSelectElement).value
                  )}
              >
                <option value="">Select…</option>
                {#each field.enum as option (option)}
                  <option value={option}>{option}</option>
                {/each}
              </select>
            {:else if field.type === 'boolean'}
              <label
                for={'boolean-value-' + key}
                class="flex cursor-pointer items-center gap-3 text-sm text-[var(--text-primary)]"
              >
                <input
                  id={'boolean-value-' + key}
                  type="checkbox"
                  class="h-4 w-4 rounded accent-[var(--accent)]"
                  disabled={readonly}
                  checked={boolValue(key)}
                  onchange={(event) =>
                    updateField(
                      key,
                      (event.currentTarget as HTMLInputElement).checked
                    )}
                />
                {field.title ?? key}
              </label>
            {:else if field.type === 'number'}
              <Input
                label={field.title ?? key}
                type="number"
                disabled={readonly}
                value={numberValue(key)?.toString() ?? ''}
                onchange={(event) => {
                  const raw = (event.currentTarget as HTMLInputElement).value
                  updateField(key, raw === '' ? null : Number(raw))
                }}
              />
            {:else if field.type === 'array' && field.items?.type === 'string'}
              <label
                for={'array-value-' + key}
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <textarea
                id={'array-value-' + key}
                class="min-h-24 w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none"
                disabled={readonly}
                value={arrayValue(key)}
                placeholder="One value per line"
                oninput={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLTextAreaElement).value
                      .split('\n')
                      .map((item) => item.trim())
                      .filter(Boolean)
                  )}
              ></textarea>
            {:else if field.type === 'object' || field.format === 'json' || field.format === 'template'}
              {@const selectedVariantType =
                key === 'params'
                  ? variantTypeMap.get(stringValue('type'))
                  : undefined}
              <div class="flex items-center justify-between">
                <label
                  for={'json-value-' + key}
                  class="block text-sm font-medium text-[var(--text-primary)]"
                >
                  {field.title ?? key}
                </label>
                {#if selectedVariantType && !readonly}
                  <button
                    type="button"
                    class="text-[11px] font-medium text-[var(--accent)] hover:underline"
                    onclick={() =>
                      updateField(key, selectedVariantType.paramsExample)}
                  >
                    Fill example
                  </button>
                {/if}
              </div>
              <textarea
                id={'json-value-' + key}
                data-testid={'json-input-' + key}
                disabled={readonly}
                value={jsonValue(key)}
                placeholder={selectedVariantType
                  ? JSON.stringify(selectedVariantType.paramsExample, null, 2)
                  : field.format === 'template'
                    ? '{{ ctx.asset_id }}'
                    : '{\n  "key": "value"\n}'}
                oninput={(event) =>
                  updateJsonField(
                    key,
                    (event.currentTarget as HTMLTextAreaElement).value
                  )}
                class="min-h-32 w-full resize-y rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 font-mono text-xs leading-relaxed text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
              ></textarea>
            {:else}
              <Input
                label={field.title ?? key}
                disabled={readonly}
                value={stringValue(key)}
                placeholder={field.placeholder}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLInputElement).value
                  )}
              />
            {/if}

            {#if field.format === 'cron'}
              <p class="text-xs text-[var(--text-muted)]">
                Standard cron expression, e.g. <code class="font-mono"
                  >0 8 * * 1-5</code
                >.
              </p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
