<script lang="ts">
</script>

<svelte:head>
  <title>Page not found — Damask docs</title>
</svelte:head>

<div class="docs-error">
  <h1>Page not found</h1>
  <p>This documentation page doesn't exist.</p>
  <a href="/docs/help">Back to docs</a>
</div>

<style>
  .docs-error {
    max-width: 480px;
    padding: 3rem 0;
  }

  .docs-error h1 {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 0.5rem;
  }

  .docs-error p {
    color: var(--text-secondary);
    margin: 0 0 1.5rem;
    line-height: 1.6;
  }

  .docs-error a {
    font-size: 0.875rem;
    color: var(--accent-cta);
    text-decoration: none;
  }

  .docs-error a:hover {
    text-decoration: underline;
  }
</style>
