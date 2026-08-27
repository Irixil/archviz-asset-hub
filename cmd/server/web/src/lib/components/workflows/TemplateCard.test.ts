import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import TemplateCard from './TemplateCard.svelte'

function renderCard(
  props: Partial<{
    icon: string
    title: string
    description: string
    badge: string | null
    loading: boolean
    disabled: boolean
    onSelect: () => void
  }> = {}
) {
  return render(TemplateCard, {
    icon: 'plus',
    title: 'Start from scratch',
    description: 'An empty workflow with a manual trigger',
    onSelect: vi.fn(),
    ...props,
  })
}

describe('TemplateCard', () => {
  it('renders title, description, and badge', () => {
    renderCard({ badge: 'Images' })
    expect(screen.getByText('Start from scratch')).toBeInTheDocument()
    expect(
      screen.getByText('An empty workflow with a manual trigger')
    ).toBeInTheDocument()
    expect(screen.getByText('Images')).toBeInTheDocument()
  })

  it('calls onSelect when clicked', async () => {
    const onSelect = vi.fn()
    renderCard({ onSelect })
    await fireEvent.click(screen.getByRole('button'))
    expect(onSelect).toHaveBeenCalledTimes(1)
  })

  it('does not call onSelect when disabled', async () => {
    const onSelect = vi.fn()
    renderCard({ onSelect, disabled: true })
    await fireEvent.click(screen.getByRole('button'))
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('shows loading state and suppresses click while loading', async () => {
    const onSelect = vi.fn()
    renderCard({ onSelect, loading: true })
    expect(screen.getByRole('button')).toBeDisabled()
    await fireEvent.click(screen.getByRole('button'))
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('is keyboard-activatable (Enter/Space)', async () => {
    const onSelect = vi.fn()
    renderCard({ onSelect })
    const button = screen.getByRole('button')
    button.focus()
    await fireEvent.keyDown(button, { key: 'Enter' })
    await fireEvent.click(button)
    expect(onSelect).toHaveBeenCalled()
  })
})
