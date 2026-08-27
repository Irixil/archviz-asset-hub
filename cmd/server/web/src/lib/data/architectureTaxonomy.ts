export type ArchiveGroupId =
  | 'assets'
  | 'projects'
  | 'references'
  | 'products'
  | 'standards'

export interface ArchitectureTaxonomyFacet {
  id: string
  label: string
  color: string
  entries: string[]
}

export interface ArchitectureTaxonomyGroup {
  id: ArchiveGroupId
  label: string
  description: string
  facets: ArchitectureTaxonomyFacet[]
  branches?: Record<string, ArchitectureTaxonomyFacet[]>
}

export const ARCHITECTURE_TAXONOMY: ArchitectureTaxonomyGroup[] = [
  {
    id: 'assets',
    label: '素材库',
    description: '可直接用于 3ds Max、V-Ray 与 Photoshop 的制作资源',
    facets: [
      {
        id: 'asset-type',
        label: '资源类型',
        color: '#2563eb',
        entries: [
          '3D模型',
          '材质贴图',
          '图片素材',
          'HDRI环境',
          '2D配景',
          'PSD素材',
          'CAD图纸',
          'IES灯光',
          '素材包',
        ],
      },
      {
        id: 'asset-space',
        label: '使用场景',
        color: '#0f766e',
        entries: ['室内空间', '室外空间', '建筑外观', '景观环境'],
      },
    ],
    branches: {
      '3D模型': [
        {
          id: 'model-category',
          label: '模型分类',
          color: '#2563eb',
          entries: [
            '家具',
            '灯具',
            '植物',
            '人物',
            '交通工具',
            '建筑构件',
            '景观小品',
            '厨卫设备',
          ],
        },
        {
          id: 'model-format',
          label: '文件格式',
          color: '#475569',
          entries: [
            'MAX模型',
            'FBX模型',
            'OBJ模型',
            'SKP模型',
            'RVT族',
            '代理模型',
          ],
        },
      ],
      材质贴图: [
        {
          id: 'material-category',
          label: '材质类别',
          color: '#0891b2',
          entries: [
            '木材',
            '石材',
            '混凝土',
            '砖瓦',
            '金属',
            '玻璃',
            '涂料',
            '瓷砖',
            '水磨石',
            '织物',
            '皮革',
            '地毯',
          ],
        },
        {
          id: 'material-position',
          label: '使用部位',
          color: '#0f766e',
          entries: ['地面', '墙面', '顶面', '建筑立面', '家具表面', '景观铺装'],
        },
        {
          id: 'material-map',
          label: '贴图通道',
          color: '#4f46e5',
          entries: [
            '基础色',
            '法线贴图',
            '粗糙度',
            '金属度',
            '置换贴图',
            '环境光遮蔽',
          ],
        },
      ],
      HDRI环境: [
        {
          id: 'hdri-scene',
          label: '环境场景',
          color: '#0369a1',
          entries: ['城市环境', '自然环境', '室内环境', '摄影棚环境'],
        },
        {
          id: 'hdri-time',
          label: '光照时段',
          color: '#d97706',
          entries: ['清晨光照', '日间光照', '黄昏光照', '夜间光照', '阴天光照'],
        },
      ],
      '2D配景': [
        {
          id: 'cutout-category',
          label: '配景分类',
          color: '#15803d',
          entries: [
            '人物配景',
            '植物配景',
            '车辆配景',
            '家具配景',
            '天空背景',
            '城市背景',
          ],
        },
        {
          id: 'cutout-style',
          label: '素材形式',
          color: '#475569',
          entries: ['透明底PNG', '剪影素材', '平面拼贴', '手绘配景'],
        },
      ],
      图片素材: [
        {
          id: 'image-category',
          label: '图片分类',
          color: '#0f766e',
          entries: [
            '效果图',
            '参考图',
            '现场照片',
            '设计过程图',
            '分析图',
            '平立剖',
            '汇报排版',
          ],
        },
        {
          id: 'image-view',
          label: '观察视角',
          color: '#475569',
          entries: [
            '人视视角',
            '鸟瞰视角',
            '仰视视角',
            '平视视角',
            '室内视角',
            '细节特写',
          ],
        },
        {
          id: 'image-atmosphere',
          label: '时间氛围',
          color: '#b45309',
          entries: ['日景', '黄昏', '夜景', '阴天', '晴天'],
        },
      ],
      PSD素材: [
        {
          id: 'psd-category',
          label: '文件用途',
          color: '#4f46e5',
          entries: [
            '效果图模板',
            '分析图模板',
            '平面彩图',
            '汇报排版',
            '图层素材',
            '调色预设',
          ],
        },
        {
          id: 'psd-layer',
          label: '图层状态',
          color: '#7c3aed',
          entries: ['完整分层', '部分合并', '单层文件'],
        },
      ],
      CAD图纸: [
        {
          id: 'cad-category',
          label: '图块分类',
          color: '#475569',
          entries: [
            '家具图块',
            '厨卫图块',
            '灯具图块',
            '门窗图块',
            '植物图块',
            '车辆图块',
            '人物图块',
            '节点图块',
          ],
        },
        {
          id: 'cad-view',
          label: '图块视图',
          color: '#334155',
          entries: ['平面图块', '立面图块', '剖面图块'],
        },
      ],
      IES灯光: [
        {
          id: 'ies-category',
          label: '灯具类型',
          color: '#b45309',
          entries: [
            '筒灯',
            '射灯',
            '线性灯',
            '吊灯',
            '壁灯',
            '洗墙灯',
            '户外灯',
          ],
        },
        {
          id: 'ies-distribution',
          label: '配光类型',
          color: '#d97706',
          entries: ['窄角配光', '中角配光', '宽角配光', '非对称配光'],
        },
      ],
    },
  },
  {
    id: 'projects',
    label: '项目档案',
    description: '按项目类型、设计阶段和交付资料归档',
    facets: [
      {
        id: 'project-type',
        label: '项目类型',
        color: '#c2410c',
        entries: [
          '住宅',
          '办公',
          '商业',
          '酒店',
          '文旅',
          '教育',
          '医疗',
          '文化建筑',
          '体育建筑',
          '交通建筑',
          '产业园区',
          '城市设计',
        ],
      },
      {
        id: 'project-stage',
        label: '设计阶段',
        color: '#b45309',
        entries: ['概念方案', '方案设计', '初步设计', '施工图设计', '竣工归档'],
      },
      {
        id: 'project-document',
        label: '项目资料',
        color: '#475569',
        entries: [
          '项目任务书',
          '设计说明',
          '项目CAD',
          '项目模型',
          '项目效果图',
          '汇报文件',
        ],
      },
    ],
  },
  {
    id: 'references',
    label: '案例参考',
    description: '按专业、建筑类型和画面表现检索参考图',
    facets: [
      {
        id: 'reference-discipline',
        label: '专业方向',
        color: '#4f46e5',
        entries: ['建筑案例', '室内案例', '景观案例', '城市设计案例'],
      },
      {
        id: 'reference-typology',
        label: '建筑类型',
        color: '#7c3aed',
        entries: [
          '住宅',
          '办公',
          '商业',
          '酒店',
          '教育',
          '文化建筑',
          '体育建筑',
          '医疗',
          '交通建筑',
          '公共空间',
        ],
      },
      {
        id: 'reference-representation',
        label: '表现形式',
        color: '#9333ea',
        entries: [
          '实景摄影',
          '效果表现',
          '分析图',
          '平面图',
          '立面图',
          '剖面图',
          '节点详图',
        ],
      },
      {
        id: 'reference-view',
        label: '观察视角',
        color: '#0369a1',
        entries: ['鸟瞰视角', '人视视角', '仰视视角', '顶视视角', '轴测视角'],
      },
      {
        id: 'reference-shot',
        label: '画面景别',
        color: '#0f766e',
        entries: ['特写', '近景', '中景', '远景'],
      },
      {
        id: 'reference-atmosphere',
        label: '时间氛围',
        color: '#d97706',
        entries: ['日景', '黄昏', '夜景', '阴天', '雨雪', '雾景'],
      },
    ],
  },
  {
    id: 'products',
    label: '产品资料',
    description: '按建筑构件系统和厂家资料类型检索',
    facets: [
      {
        id: 'product-system',
        label: '构件系统',
        color: '#0e7490',
        entries: [
          '建筑材料',
          '墙体幕墙',
          '门窗',
          '地面',
          '吊顶',
          '家具',
          '照明',
          '厨房',
          '卫浴',
          '暖通空调',
          '给排水',
          '电气',
          '消防',
          '景观产品',
          '标识导视',
        ],
      },
      {
        id: 'product-document',
        label: '资料类型',
        color: '#475569',
        entries: [
          '产品样本',
          '产品CAD',
          'BIM族库',
          '产品模型',
          '技术参数',
          '检测报告',
          '环保认证',
          '色卡样板',
        ],
      },
    ],
  },
  {
    id: 'standards',
    label: '规范知识',
    description: '按设计专业和文档类型管理规范、图集与工艺',
    facets: [
      {
        id: 'standard-discipline',
        label: '设计专业',
        color: '#334155',
        entries: [
          '建筑专业',
          '结构专业',
          '给排水专业',
          '暖通专业',
          '电气专业',
          '室内专业',
          '景观专业',
          '消防设计',
          '节能设计',
          '无障碍设计',
        ],
      },
      {
        id: 'standard-document',
        label: '文档类型',
        color: '#b91c1c',
        entries: [
          '国家标准',
          '行业标准',
          '地方标准',
          '标准图集',
          '构造节点',
          '工艺做法',
          '企业标准',
        ],
      },
    ],
  },
]

export function normalizeTaxonomyTagName(name: string) {
  return name.normalize('NFKC').trim().toLowerCase()
}

export function taxonomyGroupById(id: ArchiveGroupId | null | undefined) {
  return ARCHITECTURE_TAXONOMY.find((group) => group.id === id)
}

export function primaryFacetForGroup(group: ArchitectureTaxonomyGroup) {
  return group.facets[0]
}

function allFacetsForGroup(group: ArchitectureTaxonomyGroup) {
  return [...group.facets, ...Object.values(group.branches ?? {}).flat()]
}

export function detailFacetsForGroup(
  group: ArchitectureTaxonomyGroup,
  activeTags: Iterable<string>
) {
  const active = new Set(Array.from(activeTags, normalizeTaxonomyTagName))
  const primaryFacet = primaryFacetForGroup(group)
  const selectedPrimary = primaryFacet.entries.find((entry) =>
    active.has(normalizeTaxonomyTagName(entry))
  )

  return {
    primaryFacet,
    selectedPrimary,
    facets: [
      ...(selectedPrimary ? (group.branches?.[selectedPrimary] ?? []) : []),
      ...group.facets.slice(1),
    ],
  }
}

function taxonomySeedEntries(groups = ARCHITECTURE_TAXONOMY) {
  const entries = new Map<
    string,
    { name: string; color: string; groupName: string }
  >()

  for (const group of groups) {
    for (const facet of allFacetsForGroup(group)) {
      for (const name of facet.entries) {
        const key = normalizeTaxonomyTagName(name)
        if (!entries.has(key)) {
          entries.set(key, {
            name,
            color: facet.color,
            groupName: `${group.label} · ${facet.label}`,
          })
        }
      }
    }
  }

  return [...entries.values()]
}

export function missingTaxonomyEntries(
  existingNames: Iterable<string>,
  groupIds?: Iterable<ArchiveGroupId>
) {
  const existing = new Set(Array.from(existingNames, normalizeTaxonomyTagName))
  const selectedIds = groupIds ? new Set(groupIds) : null
  const groups = selectedIds
    ? ARCHITECTURE_TAXONOMY.filter((group) => selectedIds.has(group.id))
    : ARCHITECTURE_TAXONOMY
  return taxonomySeedEntries(groups).filter(
    (entry) => !existing.has(normalizeTaxonomyTagName(entry.name))
  )
}

export function taxonomyGroupForTag(tagName: string | undefined) {
  if (!tagName) return undefined
  const normalizedTagName = normalizeTaxonomyTagName(tagName)
  return ARCHITECTURE_TAXONOMY.find((group) =>
    allFacetsForGroup(group).some((facet) =>
      facet.entries.some(
        (entry) => normalizeTaxonomyTagName(entry) === normalizedTagName
      )
    )
  )
}
