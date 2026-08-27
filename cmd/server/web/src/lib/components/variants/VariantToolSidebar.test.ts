import type { Asset } from '$lib/api'
import { fireEvent, render, screen } from '@testing-library/svelte'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import type { VariantTab } from './VariantsTool.svelte'
import VariantToolSidebar from './VariantToolSidebar.svelte'

const baseAsset: Asset = {
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

function assetWithMime(mimeType: string): Asset {
  return { ...baseAsset, mime_type: mimeType }
}

function renderSidebar(
  props: Partial<{
    asset: Asset
    activeTool: VariantTab | null
    creating: boolean
    onSelect: (tool: VariantTab | null) => void
  }> = {}
) {
  return render(VariantToolSidebar, {
    asset: baseAsset,
    activeTool: null,
    creating: false,
    onSelect: vi.fn(),
    ...props,
  })
}

describe('VariantToolSidebar', () => {
  it('renders one button per image tool for image MIME', () => {
    renderSidebar()
    // 7 image tools + trigger_workflow (showFor: all)
    expect(screen.getAllByRole('button')).toHaveLength(8)
  })

  it('renders fewer buttons for audio MIME', () => {
    renderSidebar({ asset: assetWithMime('audio/mpeg') })
    // 2 audio tools + trigger_workflow
    expect(screen.getAllByRole('button')).toHaveLength(3)
    expect(
      screen.getByRole('button', { name: 'Transcode Audio' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Normalize' })
    ).toBeInTheDocument()
  })

  it('renders only trigger_workflow button for non-media MIME', () => {
    renderSidebar({ asset: assetWithMime('application/pdf') })
    // trigger_workflow shows for all MIME types
    expect(screen.getAllByRole('button')).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Run workflow' })
    ).toBeInTheDocument()
  })

  it('has role="toolbar"', () => {
    renderSidebar()
    expect(screen.getByRole('toolbar', { name: 'Tools' })).toBeInTheDocument()
  })

  it('marks buttons as not pressed by default', () => {
    renderSidebar()
    expect(screen.getByRole('button', { name: 'Resize' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('marks the active button as pressed', () => {
    renderSidebar({ activeTool: 'image_crop' })
    expect(screen.getByRole('button', { name: 'Crop' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  it('uses tool labels for aria-label values', () => {
    renderSidebar()
    expect(screen.getByRole('button', { name: 'Resize' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Crop' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Convert' })).toBeInTheDocument()
  })

  it('clicking an inactive tool fires onSelect with the tool key', async () => {
    const onSelect = vi.fn()
    renderSidebar({ onSelect })
    await fireEvent.click(screen.getByRole('button', { name: 'Convert' }))
    expect(onSelect).toHaveBeenCalledWith('image_convert')
  })

  it('clicking the active tool fires onSelect with null', async () => {
    const onSelect = vi.fn()
    renderSidebar({ activeTool: 'image_convert', onSelect })
    await fireEvent.click(screen.getByRole('button', { name: 'Convert' }))
    expect(onSelect).toHaveBeenCalledWith(null)
  })

  it('disables non-active buttons while creating', () => {
    renderSidebar({ activeTool: 'image_resize', creating: true })
    expect(screen.getByRole('button', { name: 'Crop' })).toBeDisabled()
  })

  it('keeps the active button enabled while creating', () => {
    renderSidebar({ activeTool: 'image_resize', creating: true })
    expect(screen.getByRole('button', { name: 'Resize' })).not.toBeDisabled()
  })

  it('ArrowDown moves focus to the next button', async () => {
    renderSidebar()
    const resize = screen.getByRole('button', { name: 'Resize' })
    const crop = screen.getByRole('button', { name: 'Crop' })

    resize.focus()
    await fireEvent.keyDown(resize, { key: 'ArrowDown' })
    expect(crop).toHaveFocus()
  })

  it('ArrowUp moves focus to the previous button', async () => {
    renderSidebar()
    const resize = screen.getByRole('button', { name: 'Resize' })
    const crop = screen.getByRole('button', { name: 'Crop' })

    crop.focus()
    await fireEvent.keyDown(crop, { key: 'ArrowUp' })
    expect(resize).toHaveFocus()
  })

  it('Enter triggers onSelect', async () => {
    const onSelect = vi.fn()
    renderSidebar({ onSelect })
    await fireEvent.keyDown(screen.getByRole('button', { name: 'Resize' }), {
      key: 'Enter',
    })
    expect(onSelect).toHaveBeenCalledWith('image_resize')
  })
})
