<script lang="ts">
  import { SortAsc, SortDesc } from '@lucide/svelte'
  type Props = {
    sort?: (key: string, asc: boolean) => void
    keys: Record<string, string>
    value: string
    asc: boolean
  }
  let { value = $bindable(), asc = $bindable(), keys, sort }: Props = $props()

  function onSort(key: string) {
    asc = key === value ? !asc : asc
    value = key
    sort?.(key, asc)
  }

  function selectSort(key: string) {
    value = key
    sort?.(key, asc)
  }
</script>

<div
  class="flex min-h-8 items-center overflow-hidden rounded-md border border-[var(--border-default)] bg-[var(--bg-app)] shadow-inner"
>
  <label class="sr-only" for="asset-sort">素材排序方式</label>
  <select
    id="asset-sort"
    class="h-8 min-w-20 appearance-none bg-transparent pr-1 pl-2.5 text-xs font-semibold text-[var(--text-primary)] outline-none"
    {value}
    onchange={(e) => selectSort(e.currentTarget.value)}
  >
    {#each Object.entries(keys) as [key, label] (key)}
      <option value={key}>{label}</option>
    {/each}
  </select>
  <button
    type="button"
    class="flex h-8 w-8 items-center justify-center border-l border-[var(--border-default)] text-[var(--accent-text)] hover:bg-[var(--bg-hover)]"
    onclick={() => onSort(value)}
    aria-label={asc ? '当前升序，点击改为降序' : '当前降序，点击改为升序'}
    title={asc ? '升序' : '降序'}
  >
    {#if asc}
      <SortAsc class="h-3.5 w-3.5" />
    {:else}
      <SortDesc class="h-3.5 w-3.5" />
    {/if}
  </button>
</div>
