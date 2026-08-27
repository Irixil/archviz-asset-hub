<script lang="ts">
  type Props = {
    lines?: number
    x?: number
    y?: number
    zoom?: number
    maxZoom?: number
  }
  let { lines = 1, x = 0, y = 0, zoom = 6, maxZoom = 20 }: Props = $props()
</script>

<div class="mb-10 space-y-3">
  {#each { length: lines } as _}
    <div class="h-20 animate-pulse rounded-xl bg-white dark:bg-gray-800"></div>
  {/each}
  {#each { length: y } as _}
    <div class="mb-6 grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
      {#each { length: x } as _}
        <div
          class="overflow-hidden rounded-xl bg-white shadow-sm dark:bg-gray-800"
        >
          <div
            class="animate-pulse bg-gray-200 dark:bg-gray-700"
            style="aspect-ratio: 4/3"
          ></div>
          <div class="p-3">
            <div
              class="mb-2 h-3 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-gray-700"
            ></div>
            <div
              class="h-3 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-700"
            ></div>
          </div>
        </div>
      {/each}
    </div>
  {/each}
</div>
