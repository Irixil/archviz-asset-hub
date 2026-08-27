<script lang="ts">
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'
  import { apiFetch } from '$lib/api'
  import AIVisionModelSelect from '$lib/components/AIVisionModelSelect.svelte'

  interface Props {
    creating?: boolean
    onCreate: (params: {
      model: string
      lang: string
      prompt: string
    }) => Promise<void>
  }

  let { creating = false, onCreate }: Props = $props()

  const DEFAULT_PROMPT = m.text_tracks_ai_description_default_prompt()

  interface SupportedLang {
    code: string
    english: string
    native: string
  }
  interface VisionModelsResponse {
    languages: SupportedLang[]
  }

  let langs = $state<SupportedLang[]>([
    { code: 'en', english: 'English', native: 'English' },
  ])

  onMount(async () => {
    const res = await apiFetch<VisionModelsResponse>('/api/v1/ai/vision-models')
    if (res.languages?.length) {
      langs = res.languages
    }
  })

  let model = $state('')
  let lang = $state('en')
  let prompt = $state(DEFAULT_PROMPT)
  let promptExpanded = $state(false)
</script>

<div class="space-y-4">
  <p class="text-sm text-[var(--text-muted)]">
    {m.text_tracks_ai_description_description()}
  </p>

  <!-- Language -->
  <label class="block space-y-2">
    <span class="text-sm font-medium text-[var(--text-primary)]">
      {m.text_tracks_ai_description_lang_label()}
    </span>
    <select
      bind:value={lang}
      class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)]"
    >
      {#each langs as { code, native }}
        <option value={code}>{native}</option>
      {/each}
    </select>
  </label>

  <!-- Model -->
  <AIVisionModelSelect bind:value={model} disabled={creating} />

  <!-- Prompt — collapsed by default -->
  <div class="space-y-2">
    <div class="flex items-center justify-between">
      <span class="text-sm font-medium text-[var(--text-primary)]">
        {m.text_tracks_ai_description_prompt_label()}
      </span>
      <button
        type="button"
        onclick={() => (promptExpanded = !promptExpanded)}
        class="text-xs text-[var(--accent-link)] hover:underline"
      >
        {promptExpanded
          ? m.text_tracks_ai_description_prompt_collapse()
          : m.text_tracks_ai_description_prompt_expand()}
      </button>
    </div>

    {#if promptExpanded}
      <textarea
        bind:value={prompt}
        rows="5"
        disabled={creating}
        class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 font-mono text-sm text-[var(--text-primary)] disabled:opacity-50"
      ></textarea>
      <button
        type="button"
        onclick={() => {
          prompt = DEFAULT_PROMPT
        }}
        class="text-xs text-[var(--text-muted)] hover:underline"
      >
        {m.text_tracks_ai_description_prompt_reset()}
      </button>
    {:else}
      <p
        class="line-clamp-2 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-xs text-[var(--text-muted)]"
      >
        {prompt}
      </p>
    {/if}
  </div>

  <button
    type="button"
    class="w-full rounded-lg bg-[var(--accent-cta)] px-4 py-2 text-sm font-medium text-white transition hover:bg-[var(--accent-cta-hover)] disabled:cursor-not-allowed disabled:opacity-50"
    disabled={creating || !model}
    onclick={() => onCreate({ model, lang, prompt })}
  >
    {creating ? m.queuing_() : m.text_tracks_ai_description_submit()}
  </button>
</div>
