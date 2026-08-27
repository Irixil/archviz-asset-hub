import type { SharedVariant } from '$lib/api'
import { fireEvent, render, screen } from '@testing-library/svelte'
import type { ComponentProps } from 'svelte'
import { describe, expect, it, vi } from 'vitest'
import SharedVariantList from './SharedVariantList.svelte'

const variants: SharedVariant[] = [
  {
    id: 'variant-1',
    title: 'Web image',
    type: 'resize',
    mime_type: 'image/jpeg',
    size: 512 * 1024,
    thumbnail_url: '/thumb-1',
    thumbnail_content_type: 'image/jpeg',
  },
  {
    id: 'variant-2',
    title: 'Print image',
    type: 'convert',
    mime_type: 'image/png',
    thumbnail_url: undefined,
    thumbnail_content_type: 'image/jpeg',
  },
]

function renderList(
  props: Partial<ComponentProps<typeof SharedVariantList>> = {}
) {
  return render(SharedVariantList, {
    shareId: 'share-1',
    assetId: 'asset-1',
    variants,
    selectedVariantId: null,
    allowDownload: true,
    getThumbUrl: (_shareId, _assetId, variantId) => `/thumb/${variantId}`,
    getDownloadUrl: (_shareId, _assetId, variantId) => `/download/${variantId}`,
    authHeaders: () => ({ 'X-Share-Token': 'token-1' }),
    ...props,
  })
}

describe('SharedVariantList', () => {
  it('renders one item per variant', () => {
    renderList()
    expect(screen.getAllByRole('button')).toHaveLength(4)
  })

  it('renders variant titles', () => {
    renderList()
    expect(screen.getByText('Web image')).toBeInTheDocument()
    expect(screen.getByText('Print image')).toBeInTheDocument()
  })

  it('renders the section title and count', () => {
    renderList()
    expect(
      screen.getByRole('heading', { name: 'Variants (2)' })
    ).toBeInTheDocument()
  })

  it('hides download buttons when downloads are disabled', () => {
    renderList({ allowDownload: false })
    expect(screen.queryByText('Download')).not.toBeInTheDocument()
  })

  it('fetches the correct download URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response('variant', {
        status: 200,
      })
    )
    vi.stubGlobal('fetch', fetchMock)
    renderList()

    await fireEvent.click(screen.getAllByText('Download')[0])

    expect(fetchMock).toHaveBeenCalledWith('/download/variant-1', {
      headers: { 'X-Share-Token': 'token-1' },
    })
  })
})
