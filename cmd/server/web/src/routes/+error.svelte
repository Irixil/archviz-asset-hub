<script lang="ts">
  import { page } from '$app/state'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import { ArrowLeft } from '@lucide/svelte'

  const status = $derived(page.status)
  const is404 = $derived(status === 404)
</script>

<svelte:head>
  <title>{status} — Damask</title>
</svelte:head>

<div class="err-root">
  <GeometricBackground />

  <!-- Content -->
  <div class="err-content">
    <!-- Status number — ghosted behind, structural not decorative -->
    <div class="err-status" aria-hidden="true">{status}</div>

    <!-- Diamond divider — Damascus tile corner motif -->
    <div class="err-divider">
      <div class="err-divider-line"></div>
      <svg width="10" height="10" viewBox="-5 -5 10 10" aria-hidden="true">
        <polygon
          points="0,-4.5 4.5,0 0,4.5 -4.5,0"
          fill="#d5d1cd"
          opacity="0.7"
        />
      </svg>
      <div class="err-divider-line"></div>
    </div>

    {#if is404}
      <h1 class="err-heading">Page not found</h1>
      <p class="err-body">
        This path doesn't exist in your library. It may have been moved or
        deleted.
      </p>
    {:else}
      <h1 class="err-heading">Something went wrong</h1>
      <p class="err-body">
        {page.error?.message ?? 'An unexpected error occurred.'}
      </p>
    {/if}

    <div class="err-actions">
      <a href="/library" class="err-btn-primary">
        <ArrowLeft size={15} />
        Back to library
      </a>
      {#if !is404}
        <button
          onclick={() => window.location.reload()}
          class="err-btn-secondary"
        >
          Reload page
        </button>
      {/if}
    </div>

    <p class="err-footer">HTTP {status} · Damask DAM</p>
  </div>
</div>

<style>
  .err-root {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
    overflow: hidden;
    background: #0e0b1f;
  }

  /* Center the star group at viewport midpoint */
  :global(.err-star-group) {
    transform: translate(50%, 50%);
  }

  /* Content layer */
  .err-content {
    position: relative;
    z-index: 10;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 0 1.5rem;
  }

  /* Ghosted status number */
  .err-status {
    font-size: clamp(5rem, 14vw, 9rem);
    font-weight: 700;
    letter-spacing: -0.05em;
    line-height: 1;
    color: #d5d1cd;
    opacity: 0.08;
    user-select: none;
    margin-bottom: -0.5rem; /* overlap below into divider space */
  }

  /* Divider */
  .err-divider {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    margin-bottom: 1.5rem;
  }

  .err-divider-line {
    width: 48px;
    height: 1px;
    background: linear-gradient(to right, transparent, #d5d1cd55);
  }
  .err-divider-line:last-child {
    background: linear-gradient(to left, transparent, #d5d1cd55);
  }

  /* Typography */
  .err-heading {
    font-size: 1.0625rem;
    font-weight: 600;
    letter-spacing: -0.01em;
    color: var(--color-gray-300);
    margin: 0 0 0.5rem;
  }

  .err-body {
    font-size: 0.8125rem;
    line-height: 1.55;
    color: var(--color-gray-300);
    max-width: 28ch;
    margin: 0;
  }

  /* Actions */
  .err-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 2rem;
  }

  .err-btn-primary {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    font-weight: 500;
    background: #d5d1cd;
    color: #0f0c08;
    text-decoration: none;
    transition:
      background 0.15s ease,
      opacity 0.15s ease;
  }
  .err-btn-primary:hover {
    background: #c9a87c;
  }
  .err-btn-primary:focus-visible {
    outline: 2px solid #d5d1cd;
    outline-offset: 2px;
  }

  .err-btn-secondary {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.8125rem;
    font-weight: 500;
    background: transparent;
    color: var(--color-gray-300);
    border: 1px solid #2a2240;
    cursor: pointer;
    transition:
      border-color 0.15s ease,
      color 0.15s ease;
  }
  .err-btn-secondary:hover {
    border-color: #d5d1cd55;
    color: #d5d1cd;
  }
  .err-btn-secondary:focus-visible {
    outline: 2px solid #d5d1cd;
    outline-offset: 2px;
  }

  /* Footer */
  .err-footer {
    margin-top: 2.5rem;
    font-family: ui-monospace, 'Cascadia Code', monospace;
    font-size: 0.625rem;
    font-weight: 500;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: #2a2240;
  }

  @media (prefers-reduced-motion: no-preference) {
    .err-btn-primary,
    .err-btn-secondary {
      transition:
        background 0.15s ease,
        border-color 0.15s ease,
        color 0.15s ease;
    }
  }
</style>
