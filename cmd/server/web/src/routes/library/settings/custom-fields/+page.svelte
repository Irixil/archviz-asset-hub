<script lang="ts">
  import { onMount } from 'svelte'
  import { GripVertical, Plus } from '@lucide/svelte'
  import { customFieldsStore } from '$lib/stores/customFields.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { FieldDefinition, FieldScope } from '$lib/api'
  import FieldCreateEditModal from '$lib/components/FieldCreateEditModal.svelte'
  import FieldDeleteModal from '$lib/components/FieldDeleteModal.svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ButtonEdit from '$lib/components/ui/ButtonEdit.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import { m } from '$lib/paraglide/messages'

  let activeScope = $state<FieldScope>('asset')

  let showCreateModal = $state(false)
  let editingField = $state<FieldDefinition | null>(null)
  let deletingField = $state<FieldDefinition | null>(null)
  let showDeleteModal = $state(false)

  // Drag-to-reorder state
  let dragIndex = $state<number | null>(null)
  let dragOverIndex = $state<number | null>(null)

  onMount(() => {
    customFieldsStore.loadBoth()
  })

  const fields = $derived(customFieldsStore.fields(activeScope))

  function handleEdit(field: FieldDefinition) {
    editingField = field
    showCreateModal = true
  }

  function handleDelete(field: FieldDefinition) {
    deletingField = field
    showDeleteModal = true
  }

  function handleSaved(def: FieldDefinition) {
    customFieldsStore.upsertLocal(activeScope, def)
    toastStore.show(editingField ? m.field_updated() : m.field_created())
    editingField = null
    showCreateModal = false
  }

  function handleDeleted(id: string) {
    customFieldsStore.removeLocal(activeScope, id)
    toastStore.show(m.field_deleted())
    deletingField = null
    showDeleteModal = false
  }

  // Drag reorder handlers
  function dragStart(i: number) {
    dragIndex = i
  }

  function dragOver(e: DragEvent, i: number) {
    e.preventDefault()
    dragOverIndex = i
  }

  async function drop() {
    if (
      dragIndex === null ||
      dragOverIndex === null ||
      dragIndex === dragOverIndex
    ) {
      dragIndex = null
      dragOverIndex = null
      return
    }
    const reordered = [...fields]
    const [moved] = reordered.splice(dragIndex, 1)
    reordered.splice(dragOverIndex, 0, moved)
    dragIndex = null
    dragOverIndex = null
    try {
      await customFieldsStore.reorder(activeScope, reordered)
    } catch {
      toastStore.show(m.reorder_failed(), 'error')
      await customFieldsStore.load(activeScope)
    }
  }

  function fieldTypeBadge(type: string): string {
    const map: Record<string, string> = {
      text: m.f_text(),
      number: m.f_number(),
      date: m.f_date(),
      boolean: m.f_bool(),
      select: m.f_select(),
      url: m.f_url(),
    }
    return map[type] ?? type
  }
</script>

<svelte:head>
  <title>{m.custom_fields_title()}｜建筑素材中台</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.custom_fields_title()}
    description={m.custom_fields_desc()}
  >
    <Button
      variant="primary"
      onclick={() => {
        editingField = null
        showCreateModal = true
      }}
    >
      {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
      {m.add_field()}
    </Button>
  </PageHeader>

  <!-- Scope tabs -->
  <div class="flex border-b border-[var(--border-subtle)] px-6">
    {#each ['asset', 'project'] as FieldScope[] as scope}
      <button
        type="button"
        class="text-md relative py-3 pr-6 font-medium transition-colors
          {activeScope === scope
          ? 'text-indigo-600 dark:text-indigo-400'
          : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'}"
        onclick={() => {
          activeScope = scope
        }}
      >
        {scope === 'asset' ? m.asset_fields() : m.project_fields()}
        {#if activeScope === scope}
          <span
            class="absolute right-4 bottom-0 left-0 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"
          ></span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- Field list -->
  <div class="mb-8 flex-1 overflow-y-auto px-6 pt-4 pb-8">
    <div class="mx-auto w-full max-w-4xl">
      {#if customFieldsStore.loading}
        <div class="flex justify-center py-16">
          <Spinner size="md" />
        </div>
      {:else if fields.length === 0}
        <EmptyState
          title="No {activeScope} fields yet"
          description="Add custom metadata fields to collect structured data on your {activeScope ===
          'asset'
            ? 'assets'
            : 'projects'}."
        />
      {:else}
        <p class="my-3 text-sm text-[var(--text-muted)]">
          {m.drag_row_reorder()}
        </p>
        <ul class="space-y-3">
          {#each fields as field, i (field.id)}
            <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
            <li
              class="flex items-center gap-3 rounded-xl border bg-[var(--bg-surface)] px-4 py-3 transition-colors
                {dragOverIndex === i
                ? 'border-indigo-400 dark:border-indigo-500'
                : 'border-[var(--border-subtle)]'}"
              draggable="true"
              ondragstart={() => dragStart(i)}
              ondragover={(e) => dragOver(e, i)}
              ondrop={drop}
              ondragend={() => {
                dragIndex = null
                dragOverIndex = null
              }}
            >
              <!-- Drag handle -->
              <GripVertical
                class="h-4 w-4 shrink-0 cursor-grab text-[var(--text-muted)] opacity-50"
              />

              <!-- Field info -->
              <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                <span class="text-md font-medium text-[var(--text-primary)]"
                  >{field.name}</span
                >
                <Badge>{fieldTypeBadge(field.field_type)}</Badge>
                <span class="font-mono text-xs text-[var(--text-muted)]"
                  >{field.key}</span
                >
                {#if field.required}
                  <span
                    class="text-xs font-medium text-orange-500 dark:text-orange-400"
                    >{m.required()}</span
                  >
                {/if}
                {#if field.inherit_from_project && activeScope === 'asset'}
                  <span class="text-xs text-[var(--text-muted)]"
                    >{m.auto_fill_assets()}</span
                  >
                {/if}
              </div>

              <!-- Options preview for select -->
              {#if field.field_type === 'select' && field.options}
                {@const opts = JSON.parse(field.options) as string[]}
                <div class="flex flex-wrap gap-1">
                  {#each opts.slice(0, 4) as opt}
                    <span
                      class="rounded bg-[var(--bg-elevated)] px-1.5 py-0.5 text-xs text-[var(--text-secondary)]"
                      >{opt}</span
                    >
                  {/each}
                  {#if opts.length > 4}
                    <span class="text-xs text-[var(--text-muted)]"
                      >+{opts.length - 4}</span
                    >
                  {/if}
                </div>
              {/if}

              <!-- Actions -->
              <div class="flex shrink-0 items-center gap-1">
                <ButtonEdit
                  title="{m.edit()} {field.name}"
                  onclick={() => handleEdit(field)}
                />
                <ButtonDelete
                  title="{m.delete()} {field.name}"
                  onclick={() => handleDelete(field)}
                />
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
</PageContainer>

<FieldCreateEditModal
  bind:open={showCreateModal}
  scope={activeScope}
  editing={editingField}
  onclose={() => {
    showCreateModal = false
    editingField = null
  }}
  onsaved={handleSaved}
/>

<FieldDeleteModal
  bind:open={showDeleteModal}
  field={deletingField}
  onclose={() => {
    showDeleteModal = false
    deletingField = null
  }}
  ondeleted={handleDeleted}
/>
