<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import type { SearchEntry } from '$lib/docs/loader'
  import { modKey } from '$lib/shortcuts/platform'

  let query = $state('')
  let index: SearchEntry[] = $state([])
  let open = $state(false)
  let focused = $state(-1)
  let inputEl: HTMLInputElement

  onMount(async () => {
    if (import.meta.env.DEV) {
      try {
        const res = await fetch('/docs-search-index.json')
        if (!res.ok) {
          console.info(
            '[DocsSearch] index not found — run a build once to enable search'
          )
          return
        }
        index = await res.json()
      } catch {
        // silently skip in dev
      }
    } else {
      const res = await fetch('/docs-search-index.json')
      if (!res.ok) return
      index = await res.json()
    }
  })

  const results = $derived.by(() => {
    if (query.length < 2) return []
    const q = query.toLowerCase()
    return index
      .filter((e) => e.title.toLowerCase().includes(q) || e.body.includes(q))
      .slice(0, 8)
  })

  $effect(() => {
    if (results.length) {
      open = true
      focused = -1
    } else {
      open = false
    }
  })

  function navigate(entry: SearchEntry) {
    const slug = entry.slug === 'index' ? '' : entry.slug
    goto(`/docs/${entry.section}/${slug}`)
    query = ''
    open = false
  }

  function onKeydown(e: KeyboardEvent) {
    if (!open) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      focused = Math.min(focused + 1, results.length - 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      focused = Math.max(focused - 1, 0)
    } else if (e.key === 'Enter' && focused >= 0) {
      e.preventDefault()
      navigate(results[focused])
    } else if (e.key === 'Escape') {
      open = false
      query = ''
    }
  }

  function globalKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault()
      inputEl?.focus()
    }
  }
</script>

<svelte:window onkeydown={globalKeydown} />

<div class="docs-search" role="search">
  <div class="docs-search-input-wrap">
    <svg
      class="docs-search-icon"
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
    >
      <circle cx="6" cy="6" r="4.5" stroke="currentColor" stroke-width="1.25" />
      <path
        d="M9.5 9.5l2.5 2.5"
        stroke="currentColor"
        stroke-width="1.25"
        stroke-linecap="round"
      />
    </svg>
    <input
      bind:this={inputEl}
      bind:value={query}
      onkeydown={onKeydown}
      onfocus={() => {
        if (results.length) open = true
      }}
      onblur={() =>
        setTimeout(() => {
          open = false
        }, 200)}
      type="search"
      class="docs-search-input"
      placeholder="Search docs…"
      autocomplete="off"
      spellcheck="false"
    />
    <kbd class="docs-search-kbd">{modKey()}K</kbd>
  </div>

  {#if open && results.length > 0}
    <ul class="docs-search-results" role="listbox">
      {#each results as entry, i}
        <li
          class="docs-search-result"
          class:active={i === focused}
          role="option"
          aria-selected={i === focused}
          onmousedown={() => navigate(entry)}
        >
          <span class="docs-search-result-section"
            >{entry.section === 'help' ? 'User guide' : 'Self-hosting'}</span
          >
          <span class="docs-search-result-title">{entry.title}</span>
        </li>
      {/each}
    </ul>
  {/if}

  {#if import.meta.env.DEV && index.length === 0 && query.length >= 2}
    <p class="docs-search-dev-hint">Build once to enable search</p>
  {/if}
</div>

<style>
  .docs-search {
    position: relative;
  }

  .docs-search-input-wrap {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.3125rem 0.5rem;
    transition: border-color 0.12s ease;
  }

  .docs-search-input-wrap:focus-within {
    border-color: var(--accent);
  }

  .docs-search-icon {
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .docs-search-input {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    outline: none;
    font-size: 0.8125rem;
    color: var(--text-primary);
    line-height: 1;
  }

  .docs-search-input::placeholder {
    color: var(--text-muted);
  }

  /* hide browser's native search cancel button */
  .docs-search-input::-webkit-search-cancel-button {
    display: none;
  }

  .docs-search-kbd {
    font-size: 0.625rem;
    color: var(--text-muted);
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    padding: 1px 4px;
    flex-shrink: 0;
    font-family: inherit;
  }

  .docs-search-results {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    list-style: none;
    margin: 0;
    padding: 0.25rem;
    z-index: 50;
    max-height: 320px;
    overflow-y: auto;
  }

  .docs-search-result {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 0.5rem 0.625rem;
    border-radius: 6px;
    cursor: pointer;
  }

  .docs-search-result:hover,
  .docs-search-result.active {
    background: var(--bg-hover);
  }

  .docs-search-result-section {
    font-size: 0.6875rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .docs-search-result-title {
    font-size: 0.8125rem;
    color: var(--text-primary);
    font-weight: 500;
  }

  .docs-search-dev-hint {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    font-size: 0.75rem;
    color: var(--text-muted);
    text-align: center;
    padding: 0.5rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
  }
</style>
