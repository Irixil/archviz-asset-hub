<script lang="ts">
  interface Props {
    label: string
    value: string | number | boolean | null | undefined
  }

  let { label, value }: Props = $props()
</script>

<dt class="text-sm text-[var(--text-muted)]">{label}</dt>
<dd
  class:text-[var(--text-muted)]={value == null}
  class="text-sm font-medium text-[var(--text-primary)]"
>
  {value ?? '—'}
</dd>
