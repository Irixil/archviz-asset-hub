<script lang="ts">
  import { tick, type Snippet } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    children: Snippet
    open?: boolean
    onclose: () => void
  }

  let { children, open = false, onclose }: Props = $props()

  let panelEl = $state<HTMLElement | null>(null)
  const focusableSelectors =
    'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'

  async function focusFirst() {
    await tick()
    panelEl?.querySelector<HTMLElement>(focusableSelectors)?.focus()
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return
    if (event.key === 'Escape') {
      onclose()
      return
    }
    if (event.key !== 'Tab' || !panelEl) return

    const focusable = Array.from(
      panelEl.querySelectorAll<HTMLElement>(focusableSelectors)
    )
    if (focusable.length === 0) return

    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    const active = document.activeElement as HTMLElement | null

    if (event.shiftKey && active === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && active === last) {
      event.preventDefault()
      first.focus()
    }
  }

  $effect(() => {
    if (!open) {
      document.body.style.overflow = ''
      return
    }

    document.body.style.overflow = 'hidden'
    focusFirst()

    return () => {
      document.body.style.overflow = ''
    }
  })
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <button
    type="button"
    class="drawer-scrim"
    onclick={onclose}
    aria-label={m.mobile_drawer_label()}
  ></button>

  <div
    bind:this={panelEl}
    class="drawer-panel"
    role="dialog"
    aria-modal="true"
    aria-label={m.mobile_drawer_label()}
    tabindex="-1"
  >
    {@render children()}
  </div>
{/if}

<style>
  .drawer-scrim {
    position: fixed;
    inset: 0;
    z-index: 50;
    border: 0;
    background: rgb(12 19 32 / 0.4);
    padding: 0;
  }

  .drawer-panel {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: 60;
    width: 75vw;
    max-width: 75vw;
    overflow: auto;
    background: var(--bg-sidebar);
    box-shadow: 0 20px 50px rgb(0 0 0 / 0.22);
    transform: translateX(0);
    animation: drawer-enter 200ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .drawer-panel :global(aside) {
    width: 100%;
    min-height: 100%;
  }

  @keyframes drawer-enter {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .drawer-panel {
      animation: none;
    }
  }
</style>
