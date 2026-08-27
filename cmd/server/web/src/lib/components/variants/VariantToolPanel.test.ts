import type { Asset } from '$lib/api'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import VariantToolPanel from './VariantToolPanel.svelte'
import type { VariantTab } from './VariantsTool.svelte'

vi.mock('$lib/api/workflows', () => ({
  workflowsApi: {
    listManual: vi.fn().mockResolvedValue([]),
    triggerBulk: vi.fn(),
  },
}))

vi.mock('$lib/stores/assets.svelte', () => ({
  assetsStore: { assets: [] },
}))

vi.mock('$app/navigation', () => ({
  goto: vi.fn(),
}))

const asset: Asset = {
  id: 'asset-1',
  workspace_id: 'workspace-1',
  original_filename: 'image.jpg',
  mime_type: 'image/jpeg',
  size: 1024,
  width: 1200,
  height: 800,
  tags: [],
  version_count: 1,
  variant_count: 0,
  variants_rebuilding: false,
  created_at: '2026-05-19T00:00:00Z',
  updated_at: '2026-05-19T00:00:00Z',
}

beforeAll(() => {
  const elementPrototype = Element.prototype as unknown as {
    animate?: typeof Element.prototype.animate
  }
  if (elementPrototype.animate) return
  elementPrototype.animate = vi.fn(() => {
    const animation = {
      cancel: vi.fn(),
      commitStyles: vi.fn(),
      finished: Promise.resolve(),
      onfinish: null as ((event: AnimationPlaybackEvent) => void) | null,
      play: vi.fn(),
    }
    setTimeout(() => animation.onfinish?.({} as AnimationPlaybackEvent), 0)
    return animation as unknown as Animation
  })
})

function renderPanel(
  props: Partial<{
    tool: VariantTab
    creating: boolean
    handleCreate: (type: string, params: object) => void
    onClose: () => void
    onApplied: (results: unknown[]) => void
  }> = {}
) {
  return render(VariantToolPanel, {
    tool: 'image_resize',
    asset,
    creating: false,
    handleCreate: vi.fn(),
    onClose: vi.fn(),
    ...props,
  })
}

describe('VariantToolPanel', () => {
  it('renders the tool label in the header', () => {
    renderPanel({ tool: 'image_resize' })
    expect(screen.getByRole('heading', { name: 'Resize' })).toBeInTheDocument()
  })

  it('renders the tool subtitle in the header', () => {
    renderPanel({ tool: 'image_resize' })
    expect(screen.getByText('Change dimensions')).toBeInTheDocument()
  })

  it('renders a close button', () => {
    renderPanel()
    expect(
      screen.getByRole('button', { name: 'Close tool panel' })
    ).toBeInTheDocument()
  })

  it('renders the VariantsTool component', () => {
    renderPanel({ tool: 'image_resize' })
    expect(screen.getByLabelText('Width (px)')).toBeInTheDocument()
  })

  it('close button click fires onClose', async () => {
    const onClose = vi.fn()
    renderPanel({ onClose })
    await fireEvent.click(
      screen.getByRole('button', { name: 'Close tool panel' })
    )
    expect(onClose).toHaveBeenCalled()
  })

  it('close button is disabled while creating', () => {
    renderPanel({ creating: true })
    expect(
      screen.getByRole('button', { name: 'Close tool panel' })
    ).toBeDisabled()
  })
})

describe('VariantToolPanel — trigger_workflow branch', () => {
  it('renders ApplyWorkflowPicker, not VariantsTool, for trigger_workflow', async () => {
    renderPanel({ tool: 'trigger_workflow' })
    await waitFor(() =>
      expect(screen.queryByLabelText('Width (px)')).not.toBeInTheDocument()
    )
  })

  it('does not render VariantsTool for trigger_workflow', () => {
    renderPanel({ tool: 'trigger_workflow' })
    expect(screen.queryByLabelText('Width (px)')).not.toBeInTheDocument()
  })

  it('calls onApplied when picker fires onApplied', async () => {
    // listManual returns one workflow so user can check and apply
    const { workflowsApi } = await import('$lib/api/workflows')
    vi.mocked(workflowsApi.listManual).mockResolvedValueOnce([
      {
        id: 'wf-1',
        workspace_id: 'ws-1',
        name: 'My workflow',
        description: 'desc',
        enabled: true,
        trigger_type: 'manual',
        graph: '{}',
        notify_on_failure_email: '',
        last_run_at: undefined,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ])
    vi.mocked(workflowsApi.triggerBulk).mockResolvedValueOnce({
      run_ids: ['run-1'],
      count: 1,
    })
    const onApplied = vi.fn()
    renderPanel({ tool: 'trigger_workflow', onApplied })
    await waitFor(() =>
      expect(screen.getByText('My workflow')).toBeInTheDocument()
    )
    await fireEvent.click(screen.getByRole('checkbox'))
    await fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    await waitFor(() => expect(onApplied).toHaveBeenCalledTimes(1))
  })

  it('shows trigger_workflow label from toolDefs in panel header', () => {
    renderPanel({ tool: 'trigger_workflow' })
    expect(
      screen.getByRole('heading', { name: 'Run workflow' })
    ).toBeInTheDocument()
  })
})
