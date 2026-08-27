import { describe, expect, it } from 'vitest'
import {
  assetFormatLabel,
  assetPrimaryCategory,
  assetTypeLabel,
  canGenerateThumbnail,
  planAssetUploads,
  setAssetDragData,
  thumbnailFileError,
} from './assetFiles'

describe('architecture asset files', () => {
  it.each([
    ['scene.max', 'application/octet-stream', '3D模型', 'MAX'],
    ['facade.FBX', 'application/octet-stream', '3D模型', 'FBX'],
    ['render.psd', 'image/vnd.adobe.photoshop', 'PSD素材', 'PSD'],
    ['plan.dwg', 'application/octet-stream', 'CAD图纸', 'DWG'],
    ['spotlight.ies', 'application/octet-stream', 'IES灯光', 'IES'],
    ['preview.jpg', 'image/jpeg', '图片素材', 'JPG'],
    ['materials.zip', 'application/zip', '素材包', 'ZIP'],
  ])('classifies %s', (filename, mime, type, format) => {
    expect(assetTypeLabel(filename, mime)).toBe(type)
    expect(assetFormatLabel(filename, mime)).toBe(format)
  })

  it('uses the same primary category names as the library filters', () => {
    expect(assetPrimaryCategory('scene.max', 'application/octet-stream')).toBe(
      '3D模型'
    )
    expect(assetPrimaryCategory('notes.txt', 'text/plain')).toBeUndefined()
  })

  it('does not leave unsupported architecture files waiting for thumbnails', () => {
    expect(canGenerateThumbnail('scene.max', 'application/octet-stream')).toBe(
      false
    )
    expect(
      canGenerateThumbnail('render.psd', 'image/vnd.adobe.photoshop')
    ).toBe(false)
    expect(canGenerateThumbnail('preview.jpg', 'image/jpeg')).toBe(true)
  })

  it('pairs an exact-name image with a MAX file as its thumbnail', () => {
    const model = new File(['max'], '大厅沙发.max')
    const preview = new File(['jpg'], '大厅沙发.jpg', {
      type: 'image/jpeg',
    })

    expect(planAssetUploads([preview, model])).toEqual([
      { file: model, thumbnail: preview },
    ])
  })

  it('keeps unrelated images as independent assets', () => {
    const model = new File(['max'], 'chair.max')
    const preview = new File(['jpg'], 'table.jpg', { type: 'image/jpeg' })

    expect(planAssetUploads([model, preview])).toEqual([
      { file: model, thumbnail: undefined },
      { file: preview, thumbnail: undefined },
    ])
  })

  it('validates manual thumbnail format and size', () => {
    expect(
      thumbnailFileError(new File(['png'], 'cover.png', { type: 'image/png' }))
    ).toBeNull()
    expect(thumbnailFileError(new File(['svg'], 'cover.svg'))).toContain('JPG')
    expect(
      thumbnailFileError({
        name: 'cover.jpg',
        size: 10 * 1024 * 1024 + 1,
      } as File)
    ).toContain('10 MB')
  })

  it('hands the shared file key to the desktop shell for native drag', () => {
    let dragged: { storageKey: string } | undefined
    let prevented = false
    window.archvizDesktop = {
      isAvailable: true,
      startDrag: (asset) => {
        dragged = asset
      },
    }

    const mode = setAssetDragData(
      {
        preventDefault: () => {
          prevented = true
        },
      } as DragEvent,
      {
        id: 'asset-1',
        original_filename: 'chair.max',
        mime_type: 'application/octet-stream',
        storage_key: 'workspace/asset/chair.max',
      },
      '/api/v1/assets/asset-1/file'
    )

    expect(mode).toBe('native')
    expect(prevented).toBe(true)
    expect(dragged).toEqual({
      storageKey: 'workspace/asset/chair.max',
    })
    delete window.archvizDesktop
  })
})
