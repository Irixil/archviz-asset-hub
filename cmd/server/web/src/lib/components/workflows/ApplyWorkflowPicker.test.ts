import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ApplyWorkflowPicker from './ApplyWorkflowPicker.svelte'

vi.mock('$lib/api/workflows', () => ({
  workflowsApi: {
    listManual: vi.fn(),
    triggerBulk: vi.fn(),
  },
}))

import { workflowsApi } from '$lib/api/workflows'

const mockListManual = vi.mocked(workflowsApi.listManual)
const mockTriggerBulk = vi.mocked(workflowsApi.triggerBulk)

const workflows = [
  {
    id: 'wf-1',
    workspace_id: 'ws-1',
    name: 'Resize images',
    description: 'Batch resize to 800px',
    enabled: true,
    trigger_type: 'manual',
    graph: '{}',
    notify_on_failure_email: '',
    last_run_at: undefined,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'wf-2',
    workspace_id: 'ws-1',
    name: 'Convert to WebP',
    description: 'Convert all images to WebP',
    enabled: true,
    trigger_type: 'manual',
    graph: '{}',
    notify_on_failure_email: '',
    last_run_at: undefined,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
]

function renderPicker(
  props: Partial<{
    assetIds: string[]
    onApplied: (results: unknown[]) => void
  }> = {}
) {
  return render(ApplyWorkflowPicker, {
    assetIds: ['asset-1'],
    onApplied: vi.fn(),
    ...props,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ApplyWorkflowPicker — loading', () => {
  it('shows spinner while listManual is pending', () => {
    mockListManual.mockReturnValue(new Promise(() => {}))
    renderPicker()
    expect(screen.getByLabelText('Loading')).toBeInTheDocument()
  })
})

describe('ApplyWorkflowPicker — error', () => {
  it('shows error message when fetch rejects', async () => {
    mockListManual.mockRejectedValue(new Error('network'))
    renderPicker()
    await waitFor(() =>
      expect(screen.getByText(/Failed to load workflows/i)).toBeInTheDocument()
    )
  })

  it('shows retry button when fetch rejects', async () => {
    mockListManual.mockRejectedValue(new Error('network'))
    renderPicker()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
    )
  })
})

describe('ApplyWorkflowPicker — empty state', () => {
  it('shows empty state message when list returns []', async () => {
    mockListManual.mockResolvedValue([])
    renderPicker()
    await waitFor(() =>
      expect(
        screen.getByText(/No manual workflows available/i)
      ).toBeInTheDocument()
    )
  })

  it('shows link to /library/settings/workflows in empty state', async () => {
    mockListManual.mockResolvedValue([])
    renderPicker()
    await waitFor(() => {
      const link = screen.getByRole('link')
      expect(link).toHaveAttribute('href', '/library/settings/workflows')
    })
  })
})

describe('ApplyWorkflowPicker — list', () => {
  it('renders one checkbox row per workflow', async () => {
    mockListManual.mockResolvedValue(workflows)
    renderPicker()
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
  })

  it('renders workflow name and description per row', async () => {
    mockListManual.mockResolvedValue(workflows)
    renderPicker()
    await waitFor(() => {
      expect(screen.getByText('Resize images')).toBeInTheDocument()
      expect(screen.getByText('Batch resize to 800px')).toBeInTheDocument()
      expect(screen.getByText('Convert to WebP')).toBeInTheDocument()
    })
  })
})

describe('ApplyWorkflowPicker — selection', () => {
  it('Apply button is disabled when no checkboxes checked', async () => {
    mockListManual.mockResolvedValue(workflows)
    renderPicker()
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled()
  })

  it('Apply button is enabled after checking one workflow', async () => {
    mockListManual.mockResolvedValue(workflows)
    renderPicker()
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    expect(screen.getByRole('button', { name: /apply/i })).not.toBeDisabled()
  })

  it('Apply button label reflects checked count and asset count', async () => {
    mockListManual.mockResolvedValue(workflows)
    renderPicker({ assetIds: ['asset-1', 'asset-2'] })
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    await fireEvent.click(screen.getAllByRole('checkbox')[1])
    expect(
      screen.getByRole('button', { name: /2 workflow.*2 asset/i })
    ).toBeInTheDocument()
  })
})

describe('ApplyWorkflowPicker — apply', () => {
  it('calls triggerBulk once per checked workflow on Apply click', async () => {
    mockListManual.mockResolvedValue(workflows)
    mockTriggerBulk.mockResolvedValue({ run_ids: ['run-1'], count: 1 })
    renderPicker()
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    await fireEvent.click(screen.getAllByRole('checkbox')[1])
    await fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    await waitFor(() => expect(mockTriggerBulk).toHaveBeenCalledTimes(2))
  })

  it('calls onApplied with correct results on full success', async () => {
    mockListManual.mockResolvedValue(workflows)
    mockTriggerBulk
      .mockResolvedValueOnce({ run_ids: ['run-1'], count: 1 })
      .mockResolvedValueOnce({ run_ids: ['run-2'], count: 1 })
    const onApplied = vi.fn()
    renderPicker({ onApplied })
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    await fireEvent.click(screen.getAllByRole('checkbox')[1])
    await fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    await waitFor(() => expect(onApplied).toHaveBeenCalledTimes(1))
    expect(onApplied).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({ workflowId: 'wf-1', runIds: ['run-1'] }),
        expect.objectContaining({ workflowId: 'wf-2', runIds: ['run-2'] }),
      ])
    )
  })

  it('shows inline error on partial failure; onApplied called for succeeded only', async () => {
    mockListManual.mockResolvedValue(workflows)
    mockTriggerBulk
      .mockResolvedValueOnce({ run_ids: ['run-1'], count: 1 })
      .mockRejectedValueOnce(new Error('failed'))
    const onApplied = vi.fn()
    renderPicker({ onApplied })
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    await fireEvent.click(screen.getAllByRole('checkbox')[1])
    await fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    await waitFor(() => expect(onApplied).toHaveBeenCalledTimes(1))
    expect(onApplied).toHaveBeenCalledWith([
      expect.objectContaining({ workflowId: 'wf-1' }),
    ])
    expect(
      screen.getByText(/Some assets could not be queued/i)
    ).toBeInTheDocument()
  })

  it('disables checkboxes and Apply button while applying', async () => {
    mockListManual.mockResolvedValue(workflows)
    let resolve!: (v: { run_ids: string[]; count: number }) => void
    mockTriggerBulk.mockReturnValue(
      new Promise((r) => {
        resolve = r
      })
    )
    renderPicker()
    await waitFor(() => expect(screen.getAllByRole('checkbox')).toHaveLength(2))
    await fireEvent.click(screen.getAllByRole('checkbox')[0])
    await fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    expect(screen.getAllByRole('checkbox')[0]).toBeDisabled()
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled()
    resolve({ run_ids: [], count: 0 })
  })
})
