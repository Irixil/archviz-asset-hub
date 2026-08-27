export type CategoryKey = 'image' | 'video' | 'audio' | 'document'

export const CATEGORY_ORDER: CategoryKey[] = [
  'image',
  'video',
  'audio',
  'document',
]

export const CATEGORY_LABELS: Record<CategoryKey, string> = {
  image: '图片与图形',
  video: '视频素材',
  audio: '音频素材',
  document: '文档与源文件',
}

export const ASSET_BACKGROUND_COLORS: Record<string, string> = {
  image: `bg-sky-300 dark:bg-sky-700`,
  video: `bg-red-300 dark:bg-red-700`,
  audio: `bg-emerald-300 dark:bg-emerald-700`,
  document: `bg-blue-200 dark:bg-blue-700`,
}

export const DOT_COLORS: Record<string, string> = {
  image: `bg-sky-200 dark:bg-sky-600`,
  video: `bg-red-200 dark:bg-red-600`,
  audio: `bg-emerald-200 dark:bg-emerald-600`,
  document: `bg-blue-100 dark:bg-blue-600`,
}

export const CATEGORY_ICON_BG: Record<
  CategoryKey,
  { light: string; dark: string }
> = {
  image: {
    light: `bg-sky-100 text-sky-600`,
    dark: `dark:bg-sky-900/50 dark:text-sky-300`,
  },
  video: {
    light: `bg-red-100 text-red-600`,
    dark: `dark:bg-red-900/50 dark:text-red-300`,
  },
  audio: {
    light: `bg-emerald-100 text-emerald-600`,
    dark: `dark:bg-emerald-900/50 dark:text-emerald-300`,
  },
  document: {
    light: `bg-blue-100 text-blue-600`,
    dark: `dark:bg-blue-900/50 dark:text-blue-300`,
  },
}

export const DOWNLOAD_BUTTON_COLORS: Record<string, string> = {
  image: 'bg-sky-500 hover:bg-sky-600 dark:bg-sky-600 dark:hover:bg-sky-500',
  video: 'bg-red-500 hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-500',
  audio:
    'bg-emerald-500 hover:bg-emerald-600 dark:bg-emerald-600 dark:hover:bg-emerald-500',
  document:
    'bg-blue-500 hover:bg-blue-600 dark:bg-blue-600 dark:hover:bg-blue-500',
}

export const CATEGORY_BORDER: Record<CategoryKey, string> = {
  image: `border-sky-200 dark:border-sky-700`,
  video: `border-red-200 dark:border-red-700`,
  audio: `border-emerald-200 dark:border-emerald-700`,
  document: `border-blue-200 dark:border-blue-700`,
}

export function getProjectColor(
  project?: { color?: string | null } | null,
  opacity = 'ff'
): string {
  return `${project?.color ?? '#9ca3af'}${opacity}`
}
