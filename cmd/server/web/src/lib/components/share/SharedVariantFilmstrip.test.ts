import type { SharedVariant } from '$lib/api'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import type { ComponentProps } from 'svelte'
import { describe, expect, it, vi } from 'vitest'
import SharedVariantFilmstrip from './SharedVariantFilmstrip.svelte'

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
    size: 2.5 * 1024 * 1024,
    thumbnail_url: undefined,
    thumbnail_content_type: 'image/jpeg',
  },
]

function renderFilmstrip(
  props: Partial<ComponentProps<typeof SharedVariantFilmstrip>> = {}
) {
  return render(SharedVariantFilmstrip, {
    shareId: 'share-1',
    assetId: 'asset-1',
    variants,
    selectedIndex: 0,
    allowDownload: true,
    getThumbUrl: (_shareId, _assetId, variantId) => `/thumb/${variantId}`,
    getDownloadUrl: (_shareId, _assetId, variantId) => `/download/${variantId}`,
    authHeaders: () => ({ 'X-Share-Token': 'token-1' }),
    ...props,
  })
}

function okResponse(headers: Record<string, string> = {}) {
  return new Response('variant', {
    status: 200,
    headers,
  })
}

describe('SharedVariantFilmstrip', () => {
  it('renders one card per variant', () => {
    renderFilmstrip()
    expect(screen.getAllByRole('option')).toHaveLength(2)
  })

  it('renders the section title', () => {
    renderFilmstrip()
    expect(
      screen.getByRole('heading', { name: 'Variants' })
    ).toBeInTheDocument()
  })

  it('renders a singular count badge', () => {
    renderFilmstrip({ variants: [variants[0]] })
    expect(screen.getByText('1 variant')).toBeInTheDocument()
  })

  it('renders a plural count badge', () => {
    renderFilmstrip()
    expect(screen.getByText('2 variants')).toBeInTheDocument()
  })

  it('shows the initial viewing counter', () => {
    renderFilmstrip()
    expect(screen.getByText('Viewing 1 of 2')).toBeInTheDocument()
  })

  it('uses the provided thumbnail URL', () => {
    renderFilmstrip()
    expect(screen.getByRole('img', { name: 'Web image' })).toHaveAttribute(
      'src',
      '/thumb/variant-1'
    )
  })

  it('renders dimensions through the fixed thumbnail frame', () => {
    renderFilmstrip()
    expect(
      screen.getAllByRole('option')[0].querySelector('.thumb')
    ).toBeTruthy()
  })

  it('renders a fallback when dimensions or thumbnails are absent', () => {
    renderFilmstrip()
    expect(screen.getByText('convert')).toBeInTheDocument()
  })

  it('formats KB sizes', () => {
    renderFilmstrip()
    expect(screen.getByText('image/jpeg · 512 KB')).toBeInTheDocument()
  })

  it('formats MB sizes', () => {
    renderFilmstrip()
    expect(screen.getByText('image/png · 2.5 MB')).toBeInTheDocument()
  })

  it('selects the first card by default', () => {
    renderFilmstrip()
    expect(screen.getAllByRole('option')[0]).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  it('fires onSelect on click', async () => {
    const onSelect = vi.fn()
    renderFilmstrip({ onSelect })
    await fireEvent.click(screen.getAllByRole('option')[1])
    expect(onSelect).toHaveBeenCalledWith(1)
  })

  it('fires onSelect on Enter', async () => {
    const onSelect = vi.fn()
    renderFilmstrip({ onSelect })
    await fireEvent.keyDown(screen.getAllByRole('option')[1], { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith(1)
  })

  it('fires onSelect on Space', async () => {
    const onSelect = vi.fn()
    renderFilmstrip({ onSelect })
    await fireEvent.keyDown(screen.getAllByRole('option')[1], { key: ' ' })
    expect(onSelect).toHaveBeenCalledWith(1)
  })

  it('updates the viewing counter on selectedIndex rerender', async () => {
    const view = renderFilmstrip()
    await view.rerender({ selectedIndex: 1 })
    expect(screen.getByText('Viewing 2 of 2')).toBeInTheDocument()
  })

  it('renders download buttons when downloads are allowed', () => {
    renderFilmstrip()
    expect(
      screen.getAllByRole('button', { name: 'Download variant' })
    ).toHaveLength(2)
  })

  it('hides download buttons when downloads are disabled', () => {
    renderFilmstrip({ allowDownload: false })
    expect(
      screen.queryByRole('button', { name: 'Download variant' })
    ).not.toBeInTheDocument()
  })

  it('fetches the correct download URL with auth headers', async () => {
    const fetchMock = vi.fn().mockResolvedValue(okResponse())
    vi.stubGlobal('fetch', fetchMock)
    renderFilmstrip()

    await fireEvent.click(
      screen.getAllByRole('button', { name: 'Download variant' })[0]
    )

    expect(fetchMock).toHaveBeenCalledWith('/download/variant-1', {
      headers: { 'X-Share-Token': 'token-1' },
    })
  })

  it('disables and re-enables the button during download', async () => {
    let resolveFetch!: (response: Response) => void
    const fetchPromise = new Promise<Response>((resolve) => {
      resolveFetch = resolve
    })
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(fetchPromise))
    renderFilmstrip()

    const button = screen.getAllByRole('button', {
      name: 'Download variant',
    })[0]
    await fireEvent.click(button)
    expect(button).toBeDisabled()

    resolveFetch(okResponse())
    await waitFor(() => expect(button).not.toBeDisabled())
  })

  it('re-enables the button after download failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network')))
    renderFilmstrip()

    const button = screen.getAllByRole('button', {
      name: 'Download variant',
    })[0]
    await fireEvent.click(button)
    await waitFor(() => expect(button).not.toBeDisabled())
  })

  it('does not select the card when download is clicked', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(okResponse()))
    const onSelect = vi.fn()
    renderFilmstrip({ onSelect })

    await fireEvent.click(
      screen.getAllByRole('button', { name: 'Download variant' })[0]
    )

    expect(onSelect).not.toHaveBeenCalled()
  })

  it('uses a Content-Disposition filename when present', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        okResponse({
          'Content-Disposition': 'attachment; filename="custom-name.jpg"',
        })
      )
    )
    const click = vi.fn()
    const createdAnchor: { current: HTMLAnchorElement | null } = {
      current: null,
    }
    vi.spyOn(document, 'createElement').mockImplementation((tagName) => {
      const element = document.createElementNS(
        'http://www.w3.org/1999/xhtml',
        tagName
      )
      if (tagName === 'a') {
        createdAnchor.current = element as HTMLAnchorElement
        createdAnchor.current.click = click
      }
      return element as HTMLElement
    })

    renderFilmstrip()
    await fireEvent.click(
      screen.getAllByRole('button', { name: 'Download variant' })[0]
    )

    await waitFor(() => expect(click).toHaveBeenCalled())
    expect(createdAnchor.current?.download).toBe('custom-name.jpg')
  })
})
