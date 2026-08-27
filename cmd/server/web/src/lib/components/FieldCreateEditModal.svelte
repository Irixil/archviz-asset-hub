<script lang="ts">
  import { fieldDefinitionApi } from '$lib/api'
  import type { FieldDefinition, FieldScope, FieldType } from '$lib/api'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import { Type, Hash, Calendar, ToggleLeft, List, Link } from '@lucide/svelte'
  import Feedback from './ui/Feedback.svelte'
  import ButtonDelete from './ui/ButtonDelete.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    open?: boolean
    scope: FieldScope
    editing?: FieldDefinition | null
    onclose: () => void
    onsaved: (def: FieldDefinition) => void
  }

  let {
    open = $bindable(false),
    scope,
    editing = null,
    onclose,
    onsaved,
  }: Props = $props()

  // Step: 'pick-type' | 'configure'
  let step = $state<'pick-type' | 'configure'>('pick-type')

  let selectedType = $state<FieldType>('text')
  let name = $state('')
  let generatedKey = $state('')
  let required = $state(false)
  let inheritFromProject = $state(false)
  // Options for select type (array of strings)
  let optionItems = $state<string[]>([''])

  let saving = $state(false)
  let error = $state('')

  // Derived auto-key from name
  function slugify(s: string): string {
    return s
      .toLowerCase()
      .replace(/[^a-z0-9\s_]/g, '')
      .replace(/[\s]+/g, '_')
      .replace(/_+/g, '_')
      .replace(/^_|_$/g, '')
  }

  $effect(() => {
    if (editing) {
      selectedType = editing.field_type as FieldType
      name = editing.name
      generatedKey = editing.key
      required = editing.required
      inheritFromProject = editing.inherit_from_project
      optionItems = editing.options
        ? (JSON.parse(editing.options) as string[])
        : ['']
      step = 'configure'
    } else {
      step = 'pick-type'
      name = ''
      generatedKey = ''
      required = false
      inheritFromProject = false
      optionItems = ['']
      error = ''
    }
  })

  function handleNameInput() {
    if (!editing) {
      generatedKey = slugify(name)
    }
  }

  const fieldTypes: { type: FieldType; label: string; description: string }[] =
    [
      { type: 'text', label: m.f_text(), description: m.f_text_desc() },
      { type: 'number', label: m.f_number(), description: m.f_number_desc() },
      { type: 'date', label: m.f_date(), description: m.f_date_desc() },
      { type: 'boolean', label: m.f_bool(), description: m.f_bool_desc() },
      { type: 'select', label: m.f_select(), description: m.f_select_desc() },
      { type: 'url', label: m.f_url(), description: m.f_url_desc() },
    ]

  function typeIcon(type: FieldType) {
    return {
      text: Type,
      number: Hash,
      date: Calendar,
      boolean: ToggleLeft,
      select: List,
      url: Link,
    }[type]
  }

  function addOption() {
    optionItems = [...optionItems, '']
  }

  function removeOption(i: number) {
    optionItems = optionItems.filter((_, idx) => idx !== i)
  }

  function updateOption(i: number, val: string) {
    optionItems = optionItems.map((o, idx) => (idx === i ? val : o))
  }

  async function handleSubmit() {
    error = ''
    if (!name.trim()) {
      error = m.name_required()
      return
    }
    if (!generatedKey) {
      error = m.key_required()
      return
    }
    if (selectedType === 'select') {
      const cleaned = optionItems.filter((o) => o.trim())
      if (cleaned.length === 0) {
        error = m.select_one_required()
        return
      }
    }

    saving = true
    try {
      let def: FieldDefinition
      if (editing) {
        const params: Parameters<typeof fieldDefinitionApi.update>[1] = {
          name: name.trim(),
          required,
          inherit_from_project: inheritFromProject,
        }
        if (selectedType === 'select') {
          params.options = JSON.stringify(optionItems.filter((o) => o.trim()))
        }
        def = await fieldDefinitionApi.update(editing.id, params)
      } else {
        def = await fieldDefinitionApi.create({
          scope,
          name: name.trim(),
          key: generatedKey,
          field_type: selectedType,
          options:
            selectedType === 'select'
              ? JSON.stringify(optionItems.filter((o) => o.trim()))
              : null,
          required,
          inherit_from_project: inheritFromProject,
        })
      }
      onsaved(def)
      open = false
      editing = null
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : m.field_save_failed()
    } finally {
      saving = false
    }
  }
</script>

<Modal bind:open onclose={() => ((editing = null), onclose())}>
  <div class="p-6">
    <h2 class="mb-5 text-base font-semibold text-gray-900 dark:text-gray-100">
      {editing
        ? 'Edit field'
        : step === 'pick-type'
          ? m.field_type_choose()
          : m.field_config()}
    </h2>

    {#if step === 'pick-type'}
      <!-- Type picker grid -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {#each fieldTypes as ft}
          {@const Icon = typeIcon(ft.type)}
          <button
            type="button"
            class="flex flex-col items-start gap-2 rounded-xl border-2 p-4 text-left transition-colors
              {selectedType === ft.type
              ? 'border-indigo-500 bg-indigo-50 dark:border-indigo-400 dark:bg-indigo-950/40'
              : 'border-gray-200 hover:border-gray-300 dark:border-gray-700 dark:hover:border-gray-600'}"
            onclick={() => {
              selectedType = ft.type
            }}
          >
            <div class="flex items-center">
              <Icon class="me-3 h-5 w-5 text-indigo-500 dark:text-indigo-400" />
              <p class="text-md font-medium text-gray-900 dark:text-gray-100">
                {ft.label}
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {ft.description}
              </p>
            </div>
          </button>
        {/each}
      </div>

      <div class="mt-5 flex justify-end gap-2">
        <Button variant="secondary" onclick={onclose}>{m.cancel()}</Button>
        <Button
          onclick={() => {
            step = 'configure'
          }}>{m.continue()}</Button
        >
      </div>
    {:else}
      <!-- Configure form -->
      <div class="space-y-4">
        <Input
          label={m.name()}
          placeholder={m.example_client_name()}
          bind:value={name}
          oninput={handleNameInput}
          autofocus={!editing}
        />

        <!-- Generated key (read-only after create) -->
        <div>
          <label
            for="field-generated-key"
            class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
          >
            Key
            {#if editing}
              <span class="ml-1 text-sm text-gray-400">(immutable)</span>
            {/if}
          </label>
          <div
            id="field-generated-key"
            class="text-md rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
          >
            {generatedKey || '—'}
          </div>
        </div>

        {#if editing}
          <!-- Show field type as read-only -->
          <div>
            <label
              for="field-type"
              class="text-md mb-1 block font-medium text-gray-700 dark:text-gray-300"
            >
              Type <span class="ml-1 text-sm text-gray-400">(immutable)</span>
            </label>
            <div
              id="field-type"
              class="text-md rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
            >
              {fieldTypes.find((f) => f.type === selectedType)?.label ??
                selectedType}
            </div>
          </div>
        {/if}

        <!-- Select options -->
        {#if selectedType === 'select'}
          <div>
            <label
              for={generatedKey + '-field-select-value'}
              class="text-md mb-2 block font-medium text-gray-700 dark:text-gray-300"
              >{m.f_options()}</label
            >
            <div id={generatedKey + '-field-select-value'} class="space-y-2">
              {#each optionItems as opt, i}
                <div class="flex items-center gap-2">
                  <input
                    type="text"
                    value={opt}
                    placeholder="Option {i + 1}"
                    oninput={(e) =>
                      updateOption(i, (e.target as HTMLInputElement).value)}
                    class="text-md flex-1 rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 placeholder-gray-400
                      focus:border-indigo-400 focus:ring-2 focus:ring-indigo-200 focus:outline-none
                      dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500
                      dark:focus:border-indigo-500 dark:focus:ring-indigo-900"
                  />
                  {#if optionItems.length > 1}
                    <ButtonDelete
                      title={m.delete()}
                      onclick={() => removeOption(i)}
                    />
                  {/if}
                </div>
              {/each}
            </div>
            <button
              type="button"
              class="mt-2 text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"
              onclick={addOption}
            >
              + {m.add_option()}
            </button>
          </div>
        {/if}

        <!-- Required toggle -->
        <label class="flex cursor-pointer items-center gap-3">
          <input
            type="checkbox"
            bind:checked={required}
            class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 dark:border-gray-600"
          />
          <span class="text-md text-gray-700 dark:text-gray-300"
            >{m.required()}</span
          >
        </label>

        <!-- Inherit from project (only for asset scope) -->
        {#if scope === 'asset'}
          <label class="flex cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              bind:checked={inheritFromProject}
              class="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500 dark:border-gray-600"
            />
            <span class="text-md text-gray-700 dark:text-gray-300">
              {m.f_inherit_default_project()}
              <span class="ml-1 text-sm text-gray-400"
                >{m.f_inherit_default_project_new()}</span
              >
            </span>
          </label>
        {/if}
      </div>

      <Feedback {error} />

      <div class="mt-5 flex justify-between gap-2">
        {#if !editing}
          <Button
            variant="secondary"
            onclick={() => {
              step = 'pick-type'
              error = ''
            }}>{m.back()}</Button
          >
        {:else}
          <Button variant="secondary" onclick={onclose}>{m.cancel()}</Button>
        {/if}
        <div class="flex gap-2">
          {#if editing}<!-- cancel already on left -->{:else}
            <Button variant="secondary" onclick={onclose}>{m.cancel()}</Button>
          {/if}
          <Button loading={saving} onclick={handleSubmit}>
            {editing ? m.save() : m.field_create()}
          </Button>
        </div>
      </div>
    {/if}
  </div>
</Modal>
