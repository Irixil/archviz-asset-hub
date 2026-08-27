import type { User, Workspace } from '$lib/api'
import { workspaceApi } from '$lib/api'
import { authStore } from '$lib/stores/auth.svelte'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import DuplicateDetectionSetting from './DuplicateDetectionSetting.svelte'

function makeWorkspace(mode: 'off' | 'warn' | 'block'): Workspace {
  return {
    id: 'ws-1',
    name: 'Test workspace',
    version_retention_count: 0,
    event_log_retention_days: 365,
    download_log_retention_days: 30,
    exif_keep: false,
    exif_keep_gps: false,
    locked_taxonomy: false,
    auto_tag_enabled: false,
    auto_tag_mode: 'pending',
    duplicate_detection_mode: mode,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

const user = { id: 'u-1', email: 'owner@example.com', name: 'Owner' } as User

describe('DuplicateDetectionSetting', () => {
  beforeEach(() => {
    authStore.login(user, makeWorkspace('warn'), 'owner')
  })

  afterEach(() => {
    authStore.logout()
    vi.restoreAllMocks()
  })

  it('renders the currently selected mode', () => {
    render(DuplicateDetectionSetting)
    expect((screen.getByDisplayValue('warn') as HTMLInputElement).checked).toBe(
      true
    )
  })

  it('calls PUT with the new mode when a different option is selected', async () => {
    const spy = vi
      .spyOn(workspaceApi, 'updateSettings')
      .mockResolvedValue(makeWorkspace('block'))
    render(DuplicateDetectionSetting)

    await fireEvent.click(screen.getByDisplayValue('block'))

    expect(spy).toHaveBeenCalledWith({ duplicate_detection_mode: 'block' })
  })

  it('reverts the selection and shows an error toast on a failed save', async () => {
    vi.spyOn(workspaceApi, 'updateSettings').mockRejectedValue(
      new Error('network error')
    )
    render(DuplicateDetectionSetting)

    await fireEvent.click(screen.getByDisplayValue('off'))

    await waitFor(() =>
      expect(
        (screen.getByDisplayValue('warn') as HTMLInputElement).checked
      ).toBe(true)
    )
  })

  it('disables the inputs for non-owners', () => {
    authStore.login(user, makeWorkspace('warn'), 'editor')
    render(DuplicateDetectionSetting)
    expect(screen.getByDisplayValue('off')).toBeDisabled()
    expect(screen.getByDisplayValue('warn')).toBeDisabled()
    expect(screen.getByDisplayValue('block')).toBeDisabled()
  })
})
