import type { Workflow } from '$lib/api/workflows'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TemplateGrid from './TemplateGrid.svelte'

const mockGoto = vi.fn()
vi.mock('$app/navigation', () => ({
  goto: (...args: unknown[]) => mockGoto(...args),
}))

vi.mock('$lib/api/workflows', () => ({
  workflowsApi: {
    getTemplates: vi.fn(),
    create: vi.fn(),
  },
}))

import { workflowsApi } from '$lib/api/workflows'

const mockGetTemplates = vi.mocked(workflowsApi.getTemplates)
const mockCreate = vi.mocked(workflowsApi.create)

const templates = [
  {
    id: 'blank-manual',
    name: 'Start from scratch',
    description: 'An empty workflow with a manual trigger',
    trigger_type: 'trigger.manual',
    graph: '{"nodes":[],"edges":[]}',
    icon: 'plus',
    category: '',
  },
  {
    id: 'image-resize-on-upload',
    name: 'Resize images on upload',
    description: 'Generate resized variants when an image is uploaded',
    trigger_type: 'trigger.asset_created',
    graph: '{"nodes":[],"edges":[]}',
    icon: 'image',
    category: 'Images',
  },
]

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TemplateGrid — loading', () => {
  it('shows loading state while fetching templates', () => {
    mockGetTemplates.mockReturnValue(new Promise(() => {}))
    render(TemplateGrid)
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument()
  })
})

describe('TemplateGrid — list', () => {
  it('renders one TemplateCard per returned template, including blank_manual', async () => {
    mockGetTemplates.mockResolvedValue(templates)
    render(TemplateGrid)
    await waitFor(() => expect(screen.getAllByRole('button')).toHaveLength(2))
    expect(screen.getByText('Start from scratch')).toBeInTheDocument()
    expect(screen.getByText('Resize images on upload')).toBeInTheDocument()
  })
})

describe('TemplateGrid — ordering', () => {
  it('always renders blank-manual first, regardless of API order', async () => {
    mockGetTemplates.mockResolvedValue([...templates].reverse())
    render(TemplateGrid)
    await waitFor(() => expect(screen.getAllByRole('button')).toHaveLength(2))
    const titles = screen.getAllByRole('button').map((btn) => btn.textContent)
    expect(titles[0]).toContain('Start from scratch')
  })
})

describe('TemplateGrid — error', () => {
  it('shows error state and retry button when getTemplates() rejects', async () => {
    mockGetTemplates.mockRejectedValue(new Error('network'))
    render(TemplateGrid)
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
    )
  })
})

describe('TemplateGrid — create', () => {
  it('navigates to /library/settings/workflows?workflow={id} on successful create', async () => {
    mockGetTemplates.mockResolvedValue(templates)
    mockCreate.mockResolvedValue({ id: 'wf-1' } as Workflow)
    render(TemplateGrid)
    await waitFor(() => expect(screen.getAllByRole('button')).toHaveLength(2))
    await fireEvent.click(screen.getByText('Start from scratch'))
    await waitFor(() =>
      expect(mockGoto).toHaveBeenCalledWith(
        '/library/settings/workflows?workflow=wf-1'
      )
    )
  })

  it('shows inline error on the clicked card when create() rejects, and re-enables it', async () => {
    mockGetTemplates.mockResolvedValue(templates)
    mockCreate.mockRejectedValue(new Error('boom'))
    render(TemplateGrid)
    await waitFor(() => expect(screen.getAllByRole('button')).toHaveLength(2))
    await fireEvent.click(screen.getByText('Start from scratch'))
    await waitFor(() =>
      expect(
        screen.getByText(/couldn't create this workflow/i)
      ).toBeInTheDocument()
    )
    expect(screen.getAllByRole('button')[0]).not.toBeDisabled()
  })

  it('disables other cards while one create is in flight', async () => {
    mockGetTemplates.mockResolvedValue(templates)
    let resolve!: (v: Workflow) => void
    mockCreate.mockReturnValue(
      new Promise((r) => {
        resolve = r
      })
    )
    render(TemplateGrid)
    await waitFor(() => expect(screen.getAllByRole('button')).toHaveLength(2))
    await fireEvent.click(screen.getByText('Start from scratch'))
    expect(screen.getAllByRole('button')[1]).toBeDisabled()
    resolve({ id: 'wf-1' } as Workflow)
  })
})
