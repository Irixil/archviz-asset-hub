<script lang="ts">
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'
  import { apiFetch } from '$lib/api'

  interface Props {
    value: string
    disabled?: boolean
    onchange?: (value: string) => void
  }

  let { value = $bindable(), disabled = false, onchange }: Props = $props()

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
    if (!value) {
      value = 'en'
      onchange?.(value)
    }
    const res = await apiFetch<VisionModelsResponse>('/api/v1/ai/vision-models')
    if (res.languages?.length) {
      langs = res.languages
    }
  })
</script>

<label class="block space-y-2">
  <span class="text-sm font-medium text-[var(--text-primary)]">
    {m.text_tracks_ai_description_lang_label()}
  </span>
  <select
    bind:value
    onchange={() => onchange?.(value)}
    {disabled}
    class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] disabled:opacity-50"
  >
    {#each langs as { code, native }}
      <option value={code}>{native}</option>
    {/each}
  </select>
</label>
