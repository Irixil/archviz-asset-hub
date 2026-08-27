import type { DuplicateOf } from '$lib/api/assets'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import type { ComponentProps } from 'svelte'
import { describe, expect, it, vi } from 'vitest'
import DuplicateWarningBanner from './DuplicateWarningBanner.svelte'

const liveDuplicate: DuplicateOf = {
  asset_id: 'asset-1',
  version_id: 'version-1',
  original_filename: 'hero-shot.jpg',
  is_deleted_version: false,
  storage_available: true,
  created_at: '2026-03-01T10:00:00Z',
}

function renderBanner(
  props: Partial<ComponentProps<typeof DuplicateWarningBanner>> = {}
) {
  return render(DuplicateWarningBanner, {
    duplicate: liveDuplicate,
    onKeepBoth: vi.fn(),
    onDeleteThisCopy: vi.fn().mockResolvedValue(undefined),
    ...props,
  })
}

describe('DuplicateWarningBanner', () => {
  it('shows the "matches existing" message with the filename for a live match', () => {
    renderBanner()
    expect(
      screen.getByText(
        'This file is identical to "hero-shot.jpg", already in your library.'
      )
    ).toBeInTheDocument()
  })

  it('shows the "matches deleted" message for a soft-deleted match', () => {
    renderBanner({
      duplicate: { ...liveDuplicate, is_deleted_version: true },
    })
    expect(
      screen.getByText(
        'This file is identical to "hero-shot.jpg", which was previously deleted.'
      )
    ).toBeInTheDocument()
  })

  it('shows the "View existing file" link when storage is available', () => {
    renderBanner()
    expect(
      screen.getByRole('link', { name: 'View existing file' })
    ).toHaveAttribute('href', '/library/assets/asset-1')
  })

  it('hides the "View existing file" link when storage is unavailable', () => {
    renderBanner({
      duplicate: { ...liveDuplicate, storage_available: false },
    })
    expect(
      screen.queryByRole('link', { name: 'View existing file' })
    ).not.toBeInTheDocument()
  })

  it('calls onKeepBoth when "Keep both" is clicked', async () => {
    const onKeepBoth = vi.fn()
    renderBanner({ onKeepBoth })
    await fireEvent.click(screen.getByRole('button', { name: 'Keep both' }))
    expect(onKeepBoth).toHaveBeenCalledOnce()
  })

  it('does not call onDeleteThisCopy until the confirmation is accepted', async () => {
    const onDeleteThisCopy = vi.fn().mockResolvedValue(undefined)
    renderBanner({ onDeleteThisCopy })
    await fireEvent.click(
      screen.getByRole('button', { name: 'Delete this copy' })
    )
    expect(onDeleteThisCopy).not.toHaveBeenCalled()
    expect(screen.getByText('Delete this duplicate?')).toBeInTheDocument()
  })

  it('calls onDeleteThisCopy once the confirmation dialog is confirmed', async () => {
    const onDeleteThisCopy = vi.fn().mockResolvedValue(undefined)
    renderBanner({ onDeleteThisCopy })
    await fireEvent.click(
      screen.getByRole('button', { name: 'Delete this copy' })
    )
    const confirmButtons = screen.getAllByRole('button', {
      name: 'Delete this copy',
    })
    await fireEvent.click(confirmButtons[confirmButtons.length - 1])
    expect(onDeleteThisCopy).toHaveBeenCalledOnce()
  })

  it('shows a loading state on the delete button while the deletion is in flight', async () => {
    let resolveDelete!: () => void
    const pending = new Promise<void>((resolve) => {
      resolveDelete = resolve
    })
    const onDeleteThisCopy = vi.fn().mockReturnValue(pending)
    renderBanner({ onDeleteThisCopy })

    await fireEvent.click(
      screen.getByRole('button', { name: 'Delete this copy' })
    )
    const confirmButtons = screen.getAllByRole('button', {
      name: 'Delete this copy',
    })
    await fireEvent.click(confirmButtons[confirmButtons.length - 1])

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Delete this copy' })
      ).toBeDisabled()
    )

    resolveDelete()
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Delete this copy' })
      ).not.toBeDisabled()
    )
  })

  it('shows the "deleted" badge for a soft-deleted match', () => {
    renderBanner({
      duplicate: { ...liveDuplicate, is_deleted_version: true },
    })
    expect(screen.getByText('Matches deleted file')).toBeInTheDocument()
  })

  it('shows the "unavailable" badge when storage is missing', () => {
    renderBanner({
      duplicate: { ...liveDuplicate, storage_available: false },
    })
    expect(screen.getByText('Original unavailable')).toBeInTheDocument()
  })

  it('shows the plain "duplicate" badge for a live, available match', () => {
    renderBanner()
    expect(screen.getByText('Duplicate')).toBeInTheDocument()
  })
})
