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

  interface OCRLanguage {
    code: string
    name: string
  }
  interface OCRLanguagesResponse {
    available: boolean
    languages: OCRLanguage[]
  }

  let langs = $state<OCRLanguage[]>([])
  let available = $state(true)
  let loading = $state(true)

  onMount(async () => {
    try {
      const res = await apiFetch<OCRLanguagesResponse>(
        '/api/v1/workflows/ocr-languages'
      )
      langs = res.languages
      available = res.available
      if (!value) {
        const def = langs.find((l) => l.code === 'eng') ?? langs[0]
        if (def) {
          value = def.code
          onchange?.(value)
        }
      }
    } catch {
      available = false
    } finally {
      loading = false
    }
  })
</script>

{#if !loading && !available}
  <p class="text-xs text-[var(--accent-warning)]">
    {m.workflow_ocr_unavailable_hint()}
  </p>
{/if}

<label class="block space-y-2">
  <span class="text-sm font-medium text-[var(--text-primary)]">
    {m.workflow_ocr_language_label()}
  </span>
  <select
    bind:value
    onchange={() => onchange?.(value)}
    disabled={disabled || (!loading && langs.length === 0)}
    class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] disabled:opacity-50"
  >
    {#if loading}
      <option value="">Loading…</option>
    {:else}
      {#each langs as lang}
        <option value={lang.code}>{lang.name}</option>
      {/each}
    {/if}
  </select>
</label>
