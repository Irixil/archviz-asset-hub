<script lang="ts">
  import { marked } from 'marked'
  import DocsTableOfContents from '$lib/components/docs/DocsTableOfContents.svelte'
  import type { PageData } from './$types'

  let { data }: { data: PageData } = $props()
  const html = $derived(marked.parse(data.raw) as string)
</script>

<svelte:head>
  <title>{data.entry.title} — Damask docs</title>
  <meta name="description" content={data.description} />
  <meta property="og:title" content="{data.entry.title} — Damask docs" />
  <meta property="og:description" content={data.description} />
</svelte:head>

<div class="docs-page">
  <article class="docs-content prose">
    {@html html}
  </article>
  <aside class="docs-toc-aside">
    <DocsTableOfContents source={data.raw} />
  </aside>
</div>

<style>
  .docs-page {
    display: flex;
    gap: 2rem;
    max-width: 1000px;
  }

  .docs-content {
    flex: 1;
    min-width: 0;
  }

  .docs-toc-aside {
    width: 200px;
    flex-shrink: 0;
  }
</style>
