<script lang="ts">
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    value: string
    disabled?: boolean
    onchange?: (value: string) => void
  }

  let { value = $bindable(), disabled = false, onchange }: Props = $props()

  onMount(() => {
    if (!value) {
      value = m.text_tracks_ai_description_default_prompt()
      onchange?.(value)
    }
  })
</script>

<label class="block space-y-2">
  <span class="text-sm font-medium text-[var(--text-primary)]">
    {m.text_tracks_ai_description_prompt_label()}
  </span>
  <textarea
    bind:value
    oninput={() => onchange?.(value)}
    rows="5"
    {disabled}
    class="min-h-32 w-full resize-y rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm leading-relaxed text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none disabled:opacity-50"
  ></textarea>
</label>
