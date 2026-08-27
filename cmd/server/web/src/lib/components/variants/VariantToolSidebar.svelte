<script lang="ts">
  import type { Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { fly } from 'svelte/transition'
  import { ALL_VARIANT_TOOLS, type VariantToolDef } from './toolDefs'
  import type { VariantTab } from './VariantsTool.svelte'
  import { cubicOut } from 'svelte/easing'

  interface Props {
    asset: Asset
    activeTool: VariantTab | null
    creating: boolean
    onSelect: (tool: VariantTab | null) => void
  }

  let { asset, activeTool, creating, onSelect }: Props = $props()

  const messages = m as unknown as Record<string, (() => string) | undefined>
  const msg = (key: string) => messages[key]?.() ?? key
  const tools = $derived(
    ALL_VARIANT_TOOLS.filter((tool) => tool.showFor(asset.mime_type))
  )

  function handleClick(tool: VariantToolDef) {
    onSelect(activeTool === tool.key ? null : tool.key)
  }

  function focusAdjacentButton(event: KeyboardEvent, direction: -1 | 1) {
    const current = event.currentTarget as HTMLButtonElement
    const toolbar = current.closest('[role="toolbar"]')
    const buttons = Array.from(
      toolbar?.querySelectorAll<HTMLButtonElement>('.tool-button') ?? []
    )
    const index = buttons.indexOf(current)
    if (index === -1 || buttons.length === 0) return

    event.preventDefault()
    buttons[(index + direction + buttons.length) % buttons.length]?.focus()
  }

  function handleKeydown(event: KeyboardEvent, tool: VariantToolDef) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      handleClick(tool)
      return
    }
    if (event.key === 'ArrowDown') focusAdjacentButton(event, 1)
    if (event.key === 'ArrowUp') focusAdjacentButton(event, -1)
  }
</script>

<div
  transition:fly={{
    x: -420,
    duration: 380,
    easing: cubicOut,
  }}
  class="tool-sidebar pointer-events-auto"
  role="toolbar"
  aria-label={msg('variant_tools_label')}
  tabindex="-1"
  onclick={(event) => event.stopPropagation()}
  onkeydown={(event) => event.stopPropagation()}
>
  <div class="tool-sidebar-label">{msg('variant_tools_label')}</div>

  <div class="tool-list">
    {#each tools as tool, index (tool.key)}
      {@const isActive = activeTool === tool.key}
      {@const isDisabled = creating && !isActive}
      <button
        type="button"
        class="tool-button"
        class:active={isActive}
        disabled={isDisabled}
        tabindex={isActive || (activeTool === null && index === 0) ? 0 : -1}
        onclick={() => handleClick(tool)}
        onkeydown={(event) => handleKeydown(event, tool)}
        aria-pressed={isActive}
        aria-label={msg(tool.label)}
      >
        <span class="active-bar" aria-hidden="true"></span>
        <tool.icon size={22} strokeWidth={1.5} />
        <span class="tool-tooltip" aria-hidden="true">
          <span class="tool-tooltip-title">{msg(tool.label)}</span>
          {#if tool.sublabel}
            <span class="tool-tooltip-sub">{msg(tool.sublabel)}</span>
          {/if}
        </span>
      </button>
    {/each}
  </div>
</div>

<style>
  .tool-sidebar {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    z-index: 20;
    display: flex;
    width: 60px;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    border-right: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    background: var(--surface-overlay, rgb(255 255 255 / 0.92));
    padding: 14px 6px;
    user-select: none;
    backdrop-filter: blur(16px);
  }

  :global(.dark) .tool-sidebar {
    border-right-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
    background: var(--dark-surface-overlay, rgb(17 24 39 / 0.9));
  }

  .tool-sidebar-label {
    display: none;
    margin-bottom: 12px;
    color: var(--text-muted, rgb(107 114 128));
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.12em;
    line-height: 1;
    text-transform: uppercase;
    writing-mode: vertical-rl;
  }

  :global(.dark) .tool-sidebar-label {
    color: var(--dark-text-muted, rgb(156 163 175));
  }

  .tool-list {
    display: flex;
    width: 100%;
    flex-direction: column;
    align-items: center;
    gap: 6px;
  }

  .tool-button {
    position: relative;
    display: flex;
    width: 48px;
    height: 48px;
    cursor: pointer;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 10px;
    background: transparent;
    color: var(--text-secondary, rgb(75 85 99));
    transition:
      background 120ms ease,
      color 120ms ease,
      opacity 120ms ease;
  }

  :global(.dark) .tool-button {
    color: var(--dark-text-secondary, rgb(209 213 219));
  }

  .tool-button:not(:disabled):hover {
    background: var(--surface-hover, rgb(249 250 251));
  }

  :global(.dark) .tool-button:not(:disabled):hover {
    background: var(--dark-surface-hover, rgb(31 41 55));
  }

  .tool-button.active {
    background: var(--surface-accent-subtle, rgb(238 242 255));
    color: var(--accent-indigo, rgb(79 70 229));
  }

  :global(.dark) .tool-button.active {
    background: var(--dark-surface-accent-subtle, rgb(49 46 129 / 0.34));
    color: var(--accent-indigo, rgb(129 140 248));
  }

  .tool-button:disabled {
    cursor: not-allowed;
    opacity: 0.4;
    pointer-events: none;
  }

  .active-bar {
    position: absolute;
    top: 12px;
    bottom: 12px;
    left: 3px;
    width: 3px;
    border-radius: 1.5px;
    background: transparent;
  }

  .tool-button.active .active-bar {
    background: var(--accent-indigo, rgb(79 70 229));
  }

  .tool-tooltip {
    pointer-events: none;
    position: absolute;
    left: calc(100% + 10px);
    top: 50%;
    transform: translateY(-50%) translateX(-4px);
    z-index: 50;
    display: flex;
    flex-direction: column;
    gap: 2px;
    white-space: nowrap;
    border-radius: 8px;
    border: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    background: var(--surface-overlay, rgb(255 255 255 / 0.96));
    padding: 7px 11px;
    box-shadow:
      0 4px 16px rgb(0 0 0 / 0.08),
      0 1px 4px rgb(0 0 0 / 0.06);
    opacity: 0;
    transition:
      opacity 120ms ease,
      transform 120ms ease;
  }

  :global(.dark) .tool-tooltip {
    border-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
    background: var(--dark-surface-overlay, rgb(17 24 39 / 0.96));
    box-shadow:
      0 4px 16px rgb(0 0 0 / 0.32),
      0 1px 4px rgb(0 0 0 / 0.2);
  }

  .tool-button:not(:disabled):hover .tool-tooltip,
  .tool-button:focus-visible .tool-tooltip {
    opacity: 1;
    transform: translateY(-50%) translateX(0);
  }

  .tool-tooltip-title {
    color: var(--text-primary, rgb(17 24 39));
    font-size: 14px;
    font-weight: 600;
    line-height: 1.3;
  }

  :global(.dark) .tool-tooltip-title {
    color: var(--dark-text-primary, rgb(243 244 246));
  }

  .tool-tooltip-sub {
    color: var(--text-muted, rgb(107 114 128));
    font-size: 12px;
    font-weight: 400;
    line-height: 1.3;
  }

  :global(.dark) .tool-tooltip-sub {
    color: var(--dark-text-muted, rgb(156 163 175));
  }
</style>
