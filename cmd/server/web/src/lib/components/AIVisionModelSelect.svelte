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

  interface VisionModel {
    id: string
    name: string
  }
  interface VisionModelsResponse {
    configured: boolean
    default_model: string
    models: VisionModel[]
  }

  let models = $state<VisionModel[]>([])
  let configured = $state(true)
  let loading = $state(true)

  onMount(async () => {
    try {
      const res = await apiFetch<VisionModelsResponse>(
        '/api/v1/ai/vision-models'
      )
      models = res.models
      configured = res.configured
      if (!value && res.default_model) {
        value = res.default_model
        onchange?.(value)
      }
    } finally {
      loading = false
    }
  })
</script>

{#if !loading && !configured}
  <p class="text-xs text-[var(--accent-warning)]">
    {@html m.text_tracks_ai_description_openrouter_not_configured_hint()}
  </p>
{/if}

<label class="block space-y-2">
  <span class="text-sm font-medium text-[var(--text-primary)]">
    {m.text_tracks_ai_description_model_label()}
  </span>
  <select
    bind:value
    onchange={() => onchange?.(value)}
    {disabled}
    class="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] disabled:opacity-50"
  >
    {#if loading}
      <option value="">Loading…</option>
    {:else}
      {#each models as model}
        <option value={model.id}>{model.name}</option>
      {/each}
    {/if}
  </select>
</label>
