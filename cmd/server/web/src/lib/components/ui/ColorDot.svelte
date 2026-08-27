<script lang="ts">
  interface Props {
    color: string
    size?: 'sm' | 'md' | 'lg'
    class?: string
  }

  let { color, size = 'md', class: extraClass = '' }: Props = $props()

  const sizes: Record<string, string> = {
    sm: 'h-2 w-2',
    md: 'h-3 w-3',
    lg: 'h-4 w-4',
  }
</script>

<span
  class="inline-block shrink-0 rounded-full {sizes[size]} {extraClass}"
  style="background-color: {color}"
></span>
