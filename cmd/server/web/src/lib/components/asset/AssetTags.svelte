<script lang="ts">
  import {
    tagApi,
    autoTagApi,
    type Asset,
    type AutoTagSuggestion,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import TagSuggestionChip from './TagSuggestionChip.svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { TagAsset } from '$lib/commands/TagAsset'
  import { assetsStore } from '$lib/stores/assets.svelte'

  interface Props {
    asset: Asset | null
  }

  let { asset }: Props = $props()

  // --- Asset state ---
  let tagInput = $state('')
  let tagSuggestions = $state<string[]>([])
  let showTagInput = $state(false)
  let allTags = $state<{ id: string; name: string; asset_count: number }[]>([])
  let tagError = $state('')

  // --- AI auto-tag suggestions ---
  let autoTagSuggestions = $state<AutoTagSuggestion[]>([])
  let autoTagTriggering = $state(false)
  let autoTagError = $state('')

  const autoTagEligible = $derived(
    !!asset &&
      (asset.mime_type.startsWith('image/') ||
        asset.mime_type.startsWith('video/') ||
        asset.mime_type.startsWith('audio/') ||
        asset.mime_type === 'application/pdf')
  )

  async function reloadAutoTagSuggestions() {
    if (!asset) return
    try {
      const res = await autoTagApi.listSuggestions(asset.id)
      autoTagSuggestions = res.suggestions
    } catch {
      /* silently ignore */
    }
  }

  async function acceptAutoTagSuggestion(s: AutoTagSuggestion) {
    if (!asset) return
    try {
      await autoTagApi.acceptSuggestion(asset.id, s.id)
      assetsStore.addTag(asset.id, s.tag_name)
      autoTagSuggestions = autoTagSuggestions.filter((x) => x.id !== s.id)
    } catch {
      /* silently ignore */
    }
  }

  async function dismissAutoTagSuggestion(s: AutoTagSuggestion) {
    if (!asset) return
    try {
      await autoTagApi.dismissSuggestion(asset.id, s.id)
      autoTagSuggestions = autoTagSuggestions.filter((x) => x.id !== s.id)
    } catch {
      /* silently ignore */
    }
  }

  async function acceptAllAutoTagSuggestions() {
    if (!asset) return
    const before = autoTagSuggestions
    try {
      await autoTagApi.acceptAll(asset.id)
    } catch (e) {
      autoTagError = e instanceof Error ? e.message : m.auto_tag_trigger_error()
    }
    await reloadAutoTagSuggestions()
    const remainingIds = new Set(autoTagSuggestions.map((s) => s.id))
    for (const s of before) {
      if (!remainingIds.has(s.id)) assetsStore.addTag(asset.id, s.tag_name)
    }
  }

  async function dismissAllAutoTagSuggestions() {
    if (!asset) return
    try {
      await autoTagApi.dismissAll(asset.id)
      autoTagSuggestions = []
    } catch {
      /* silently ignore */
    }
  }

  let autoTagTimer: ReturnType<typeof setTimeout> | undefined

  async function triggerAutoTag() {
    if (!asset) return
    autoTagTriggering = true
    autoTagError = ''
    try {
      await autoTagApi.trigger(asset.id)
      clearTimeout(autoTagTimer)
      autoTagTimer = setTimeout(reloadAutoTagSuggestions, 3000)
    } catch (e) {
      autoTagError = e instanceof Error ? e.message : m.auto_tag_trigger_error()
    } finally {
      autoTagTriggering = false
    }
  }

  const tags = $derived(
    (asset ? assetsStore.assets.find((a) => a.id === asset.id) : null)?.tags ??
      asset?.tags ??
      []
  )

  const noMatch = $derived(
    tagInput.trim().length > 0 && tagSuggestions.length === 0
  )
  const showCreate = $derived(noMatch && authStore.canCreateTag)
  const showVocabHint = $derived(
    noMatch && authStore.taxonomyLocked && !authStore.canCreateTag
  )

  function updateSuggestions() {
    const q = tagInput.trim().toLowerCase()
    tagError = ''
    if (!q) {
      tagSuggestions = []
      return
    }
    tagSuggestions = allTags
      .map((t) => t.name)
      .filter((n) => n.includes(q) && !tags.includes(n))
      .slice(0, 5)
  }

  async function addTag(name: string) {
    if (!asset || !name.trim()) return
    const n = name.trim().toLowerCase()
    if (tags.includes(n)) return
    tagError = ''
    try {
      await undoStore.execute(new TagAsset(asset.id, n, 'add'))
      tagInput = ''
      showTagInput = false
    } catch (e) {
      if (e instanceof Error && e.message.includes('tag_not_in_vocabulary')) {
        tagError = m.tag_vocab_locked_api_error()
      }
    }
  }

  async function removeTag(name: string) {
    if (!asset) return
    try {
      await undoStore.execute(new TagAsset(asset.id, name, 'remove'))
    } catch {
      /* silently ignore */
    }
  }

  $effect(() => {
    if (!asset) return
    tagApi
      .list()
      .then((t) => {
        allTags = t
      })
      .catch(() => {})
  })

  $effect(() => {
    if (asset && autoTagEligible) {
      autoTagSuggestions = []
      reloadAutoTagSuggestions()
    }
    return () => clearTimeout(autoTagTimer)
  })
</script>

<div>
  <SubSectionTitle>{m.tags()}</SubSectionTitle>
  <div class="flex flex-wrap gap-1.5">
    {#each tags as tag}
      <Chip
        label={tag}
        onremove={authStore.role !== 'viewer'
          ? () => removeTag(tag)
          : undefined}
      />
    {/each}

    {#if authStore.role !== 'viewer'}
      {#if showTagInput}
        <div
          class="relative"
          onfocusout={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node)) {
              showTagInput = false
              tagSuggestions = []
            }
          }}
        >
          <form
            onsubmit={(e) => {
              e.preventDefault()
              addTag(tagInput)
            }}
          >
            <input
              bind:value={tagInput}
              oninput={updateSuggestions}
              placeholder="Add tag…"
              class="w-28 rounded-full border border-indigo-400 bg-white px-2.5 py-0.5 text-sm text-gray-900 outline-none dark:bg-gray-800 dark:text-gray-100"
            />
          </form>
          {#if tagSuggestions.length > 0 || showVocabHint}
            <ul
              class="absolute top-full left-0 z-20 mt-0.5 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-md dark:border-gray-700 dark:bg-gray-900"
            >
              {#each tagSuggestions as s}
                <li>
                  <button
                    class="w-full px-3 py-1 text-left text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                    onmousedown={() => addTag(s)}>{s}</button
                  >
                </li>
              {/each}
              {#if showCreate}
                <li>
                  <button
                    class="w-full px-3 py-1 text-left text-sm text-indigo-600 hover:bg-gray-50 dark:text-indigo-400 dark:hover:bg-gray-800"
                    onmousedown={() => addTag(tagInput.trim())}
                    >+ {m.tag_new()} "{tagInput.trim()}"</button
                  >
                </li>
              {/if}
              {#if showVocabHint}
                <li
                  class="flex items-center gap-1.5 px-3 py-1 text-sm text-[var(--text-muted)]"
                >
                  🔒 {m.tag_autocomplete_not_in_vocab()}
                </li>
              {/if}
            </ul>
          {/if}
          {#if tagError}
            <p
              class="absolute top-full left-0 mt-1 w-56 rounded bg-red-50 px-2 py-1 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400"
            >
              {tagError}
            </p>
          {/if}
        </div>
      {:else}
        <button
          class="rounded-full border border-dashed border-gray-300 px-2.5 py-0.5 text-sm text-gray-400 hover:border-indigo-400 hover:text-indigo-600 dark:border-gray-600 dark:text-gray-500 dark:hover:border-indigo-500 dark:hover:text-indigo-400"
          onclick={() => {
            showTagInput = true
          }}>+ {m.tag_new()}</button
        >
      {/if}
    {/if}

    {#each autoTagSuggestions as suggestion (suggestion.id)}
      <TagSuggestionChip
        label={suggestion.tag_name}
        disabled={authStore.role === 'viewer'}
        onaccept={() => acceptAutoTagSuggestion(suggestion)}
        ondismiss={() => dismissAutoTagSuggestion(suggestion)}
      />
    {/each}
  </div>

  {#if authStore.role !== 'viewer' && autoTagEligible}
    <div class="mt-1.5 flex items-center gap-3">
      {#if autoTagSuggestions.length > 0}
        <button
          type="button"
          onclick={acceptAllAutoTagSuggestions}
          class="text-xs text-indigo-600 hover:underline dark:text-indigo-400"
        >
          {m.auto_tag_accept_all()}
        </button>
        <button
          type="button"
          onclick={dismissAllAutoTagSuggestions}
          class="text-xs text-[var(--text-muted)] hover:underline"
        >
          {m.auto_tag_dismiss_all()}
        </button>
      {:else}
        <button
          type="button"
          onclick={triggerAutoTag}
          disabled={autoTagTriggering}
          class="text-xs text-indigo-600 hover:underline disabled:opacity-50 dark:text-indigo-400"
        >
          {autoTagTriggering
            ? m.auto_tag_triggering()
            : m.auto_tag_trigger_button()}
        </button>
      {/if}
      {#if autoTagError}
        <p class="text-xs text-red-600 dark:text-red-400">{autoTagError}</p>
      {/if}
    </div>
  {/if}
</div>
