<script lang="ts">
  interface Props {
    value?: string
    disabled?: boolean
    label?: string
    id?: string
    onchange: (value: string) => void
  }

  let {
    value = '192k',
    disabled = false,
    label = 'Bitrate',
    id = 'variant-bitrate',
    onchange,
  }: Props = $props()

  const bitrates = ['64k', '96k', '128k', '192k', '256k', '320k']
</script>

<div class:opacity-50={disabled}>
  <label for={id} class="field-label">{label}</label>
  <select
    {id}
    {disabled}
    {value}
    class="field-input"
    onchange={(e) => onchange((e.currentTarget as HTMLSelectElement).value)}
  >
    {#each bitrates as bitrate}
      <option value={bitrate}>{bitrate}</option>
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
  .field-input:disabled {
    cursor: not-allowed;
  }
</style>
