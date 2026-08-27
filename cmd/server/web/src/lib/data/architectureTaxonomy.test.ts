import { describe, expect, it } from 'vitest'
import {
  ARCHITECTURE_TAXONOMY,
  detailFacetsForGroup,
  missingTaxonomyEntries,
  taxonomyGroupForTag,
} from './architectureTaxonomy'

describe('architecture taxonomy', () => {
  it('separates the five archive domains into named facets', () => {
    expect(ARCHITECTURE_TAXONOMY.map((group) => group.id)).toEqual([
      'assets',
      'projects',
      'references',
      'products',
      'standards',
    ])
    expect(
      ARCHITECTURE_TAXONOMY.find(
        (group) => group.id === 'references'
      )?.facets.map((facet) => facet.label)
    ).toEqual([
      '专业方向',
      '建筑类型',
      '表现形式',
      '观察视角',
      '画面景别',
      '时间氛围',
    ])
  })

  it('returns each missing tag once even when facets reuse it', () => {
    const missing = missingTaxonomyEntries([])
    const normalized = missing.map((tag) => tag.name.toLowerCase())
    expect(new Set(normalized).size).toBe(normalized.length)
    expect(
      missingTaxonomyEntries(['3D模型', '住宅']).map((tag) => tag.name)
    ).not.toContain('3D模型')
    expect(taxonomyGroupForTag('鸟瞰视角')?.id).toBe('assets')
  })

  it('expands material-specific categories only under material assets', () => {
    const assets = ARCHITECTURE_TAXONOMY.find((group) => group.id === 'assets')!
    const material = detailFacetsForGroup(assets, ['材质贴图'])
    const model = detailFacetsForGroup(assets, ['3D模型'])

    expect(material.selectedPrimary).toBe('材质贴图')
    expect(material.facets.map((facet) => facet.label)).toEqual([
      '材质类别',
      '使用部位',
      '贴图通道',
      '使用场景',
    ])
    expect(material.facets[0].entries).toContain('石材')
    expect(material.facets[1].entries).toContain('地面')
    expect(model.facets[0].label).toBe('模型分类')
  })

  it('keeps uploaded file categories aligned with first-stage asset filters', () => {
    const assets = ARCHITECTURE_TAXONOMY.find((group) => group.id === 'assets')!
    expect(assets.facets[0].entries).toEqual(
      expect.arrayContaining([
        '3D模型',
        '图片素材',
        'PSD素材',
        'CAD图纸',
        'IES灯光',
        '素材包',
      ])
    )
    expect(detailFacetsForGroup(assets, ['图片素材']).facets[0].label).toBe(
      '图片分类'
    )
  })

  it('recognizes existing Latin names without case sensitivity', () => {
    const missing = missingTaxonomyEntries([
      '3d模型',
      'hdri环境',
      'ies灯光',
    ]).map((tag) => tag.name)

    expect(missing).not.toContain('3D模型')
    expect(missing).not.toContain('HDRI环境')
    expect(missing).not.toContain('IES灯光')
    expect(taxonomyGroupForTag('3d模型')?.id).toBe('assets')
  })
})
