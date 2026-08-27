<script lang="ts">
  interface Props {
    value: number
    color?: string
    class?: string
  }

  let { value, color, class: extraClass = '' }: Props = $props()

  const clamped = $derived(Math.max(0, Math.min(100, value)))
</script>

<div
  class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700 {extraClass}"
  role="progressbar"
  aria-valuenow={clamped}
  aria-valuemin={0}
  aria-valuemax={100}
>
  <div
    class="h-full rounded-full transition-all duration-300 {color
      ? ''
      : 'bg-indigo-600 dark:bg-indigo-500'}"
    style="width: {clamped}%; {color ? `background-color: ${color}` : ''}"
  ></div>
</div>
