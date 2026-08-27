<script lang="ts">
  interface Props {
    value: string
    options: { value: string; label: string }[]
    label?: string
    id?: string
    onchange: (value: string) => void
  }

  let {
    value,
    options,
    label = 'Format',
    id = 'variant-format',
    onchange,
  }: Props = $props()
</script>

<div>
  <label for={id} class="field-label">{label}</label>
  <select
    {id}
    {value}
    class="field-input"
    onchange={(e) => onchange((e.currentTarget as HTMLSelectElement).value)}
  >
    {#each options as option}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
</div>

<style>
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .field-input {
    width: 100%;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-primary);
    padding: 7px 10px;
    font-size: 0.875rem;
    outline: none;
    transition: border-color 0.12s ease;
  }
  .field-input:focus {
    border-color: var(--accent-cta);
  }
</style>
