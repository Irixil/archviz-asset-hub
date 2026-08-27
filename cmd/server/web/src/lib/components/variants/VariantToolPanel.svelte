<script lang="ts">
  import type { Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { X } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { ALL_VARIANT_TOOLS, PARAM_HISTORY_TOOLS } from './toolDefs'
  import VariantsTool from './VariantsTool.svelte'
  import type { VariantTab } from './VariantsTool.svelte'
  import ApplyWorkflowPicker from '$lib/components/workflows/ApplyWorkflowPicker.svelte'
  import ParamHistoryButton from './ParamHistoryButton.svelte'
  import { formatParamSummary } from './paramHistorySummary'

  interface AppliedWorkflow {
    workflowId: string
    workflowName: string
    runIds: string[]
    partialError: string | undefined
  }

  interface Props {
    tool: VariantTab
    asset: Asset
    creating: boolean
    handleCreate: (type: string, params: object) => void
    onClose: () => void
    onDraftStarted?: (nonce: string, meta?: Record<string, unknown>) => void
    sessionActive?: boolean
    onApplied?: (results: AppliedWorkflow[]) => void
    reuseParams?: Record<string, unknown> | null
    reuseNonce?: number
    onReuseParams?: (params: Record<string, unknown>) => void
  }

  let {
    tool,
    asset,
    creating,
    handleCreate,
    onClose,
    onDraftStarted,
    sessionActive,
    onApplied,
    reuseParams = null,
    reuseNonce = 0,
    onReuseParams,
  }: Props = $props()

  const showHistoryButton = $derived(
    onReuseParams && PARAM_HISTORY_TOOLS.has(tool)
  )

  function handleDraftDone() {
    onClose()
  }

  const messages = m as unknown as Record<string, (() => string) | undefined>
  const msg = (key: string) => messages[key]?.() ?? key
  const toolDef = $derived(ALL_VARIANT_TOOLS.find((item) => item.key === tool))
</script>

<div
  class="variant-tool-panel pointer-events-auto"
  data-testid="variant-tool-panel"
  role="presentation"
  transition:fly={{ x: -16, duration: 200, opacity: 0 }}
  onclick={(event) => event.stopPropagation()}
  onkeydown={(event) => event.stopPropagation()}
>
  <header class="panel-header">
    <div class="panel-title-group">
      <h2>{toolDef ? msg(toolDef.label) : tool}</h2>
      {#if toolDef}
        <p>{msg(toolDef.sublabel)}</p>
      {/if}
    </div>
    <div class="panel-header-actions">
      {#if showHistoryButton}
        <ParamHistoryButton
          variantType={tool}
          disabled={creating}
          onSelect={(params) => onReuseParams?.(params)}
          formatEntry={(params) => formatParamSummary(tool, params)}
          dropdownWidthClass={tool === 'custom_ffmpeg' ? 'w-96' : 'w-64'}
          monospaceEntries={tool === 'custom_ffmpeg'}
        />
      {/if}
      <button
        type="button"
        class="close-button"
        disabled={creating}
        onclick={() => {
          if (!creating) onClose()
        }}
        aria-label={msg('variant_tool_panel_close')}
        title={msg('variant_tool_panel_close')}
      >
        <X size={18} strokeWidth={1.75} />
      </button>
    </div>
  </header>

  <div class="panel-body">
    {#if tool === 'trigger_workflow'}
      <ApplyWorkflowPicker assetIds={[asset.id]} {onApplied} />
    {:else}
      {#key `${tool}-${reuseNonce}`}
        <VariantsTool
          {tool}
          {asset}
          {creating}
          initialParams={reuseParams}
          {handleCreate}
          onDone={handleDraftDone}
          {onDraftStarted}
          {sessionActive}
        />
      {/key}
    {/if}
  </div>
</div>

<style>
  .variant-tool-panel {
    --tool-panel-width: 370px;

    position: absolute;
    top: 0;
    bottom: 0;
    left: 60px;
    z-index: 20;
    display: flex;
    width: var(--tool-panel-width);
    flex-direction: column;
    border-right: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    background: var(--surface-panel, rgb(255 255 255 / 0.96));
    box-shadow: 16px 0 36px rgb(15 23 42 / 0.12);
    backdrop-filter: blur(18px);
  }

  :global(.dark) .variant-tool-panel {
    border-right-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
    background: var(--dark-surface-panel, rgb(17 24 39 / 0.96));
    box-shadow: 18px 0 42px rgb(0 0 0 / 0.32);
  }

  .panel-header {
    display: flex;
    flex-shrink: 0;
    align-items: flex-start;
    gap: 12px;
    border-bottom: 1px solid var(--border-subtle, rgb(229 231 235 / 0.72));
    padding: 16px 14px 14px 18px;
  }

  :global(.dark) .panel-header {
    border-bottom-color: var(--dark-border-subtle, rgb(55 65 81 / 0.72));
  }

  .panel-title-group {
    min-width: 0;
    flex: 1;
  }

  .panel-title-group h2 {
    margin: 0;
    color: var(--text-primary, rgb(17 24 39));
    font-size: 0.95rem;
    font-weight: 700;
    line-height: 1.25;
  }

  :global(.dark) .panel-title-group h2 {
    color: var(--dark-text-primary, white);
  }

  .panel-title-group p {
    margin: 4px 0 0;
    color: var(--text-muted, rgb(107 114 128));
    font-size: 12px;
    line-height: 1.35;
  }

  :global(.dark) .panel-title-group p {
    color: var(--dark-text-muted, rgb(156 163 175));
  }

  .panel-header-actions {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: 4px;
  }

  .close-button {
    display: flex;
    width: 32px;
    height: 32px;
    flex-shrink: 0;
    cursor: pointer;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-secondary, rgb(75 85 99));
    transition:
      background 120ms ease,
      color 120ms ease,
      opacity 120ms ease;
  }

  .close-button:not(:disabled):hover {
    background: var(--surface-hover, rgb(249 250 251));
    color: var(--text-primary, rgb(17 24 39));
  }

  .close-button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }

  :global(.dark) .close-button {
    color: var(--dark-text-secondary, rgb(209 213 219));
  }

  :global(.dark) .close-button:not(:disabled):hover {
    background: var(--dark-surface-hover, rgb(31 41 55));
    color: var(--dark-text-primary, white);
  }

  .panel-body {
    min-height: 0;
    flex: 1;
    overflow: auto;
    padding: 16px 18px 20px;
  }
</style>
