import type { DocSection } from './nav'

export interface SearchEntry {
  section: 'help' | 'tech'
  slug: string
  title: string
  body: string
}

const helpDocs = import.meta.glob('./help/*.md', {
  query: '?raw',
  import: 'default',
})
const techDocs = import.meta.glob('./tech/*.md', {
  query: '?raw',
  import: 'default',
})

export async function loadDoc(
  section: DocSection,
  slug: string
): Promise<string> {
  const map = section === 'help' ? helpDocs : techDocs
  const key = `./${section}/${slug}.md`
  const loader = map[key]
  if (!loader) throw new Error(`Doc not found: ${section}/${slug}`)
  return (await loader()) as string
}

export function extractDescription(src: string): string {
  const lines = src.split('\n')
  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed && !trimmed.startsWith('#')) {
      return trimmed.slice(0, 160)
    }
  }
  return ''
}
