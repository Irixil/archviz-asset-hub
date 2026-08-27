import type { User, Workspace } from '$lib/api'
import { authStore } from '$lib/stores/auth.svelte'
import { render, screen, waitFor } from '@testing-library/svelte'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Page from './+page.svelte'

vi.mock('$app/state', () => ({
  page: { url: new URL('http://localhost/library/settings/workflows') },
}))

vi.mock('$lib/api/workflows', () => ({
  workflowsApi: {
    list: vi.fn(),
    getNodeSchemas: vi.fn(),
  },
}))

import { workflowsApi } from '$lib/api/workflows'

const mockList = vi.mocked(workflowsApi.list)
const mockGetNodeSchemas = vi.mocked(workflowsApi.getNodeSchemas)

const user: User = {
  id: 'usr-1',
  email: 'user@example.com',
  name: 'Test User',
  created_at: '2026-01-01T00:00:00Z',
}

const workspace: Workspace = {
  id: 'ws-1',
  name: 'Test workspace',
} as Workspace

beforeEach(() => {
  vi.clearAllMocks()
  mockList.mockResolvedValue([])
  mockGetNodeSchemas.mockResolvedValue([])
})

describe('workflows +page — New workflow entry point', () => {
  it('renders link to ./new instead of the add-workflow dropdown', async () => {
    authStore.login(user, workspace, 'owner')
    render(Page)
    await waitFor(() => expect(mockList).toHaveBeenCalled())
    const link = screen.getByRole('link', { name: /add workflow/i })
    expect(link).toHaveAttribute('href', '/library/settings/workflows/new')
    authStore.logout()
  })

  it('hides the New workflow link when role is not owner', async () => {
    authStore.login(user, workspace, 'editor')
    render(Page)
    await waitFor(() => expect(mockList).toHaveBeenCalled())
    expect(
      screen.queryByRole('link', { name: /add workflow/i })
    ).not.toBeInTheDocument()
    authStore.logout()
  })
})
