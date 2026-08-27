import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import type { Plugin } from 'vite'

// mirrors SearchEntry in $lib/docs/loader.ts — keep fields in sync
export interface SearchEntry {
  section: 'help' | 'tech'
  slug: string
  title: string
  body: string
}

function stripMd(src: string): string {
  return src
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/\*\*(.+?)\*\*/g, '$1')
    .replace(/\*(.+?)\*/g, '$1')
    .replace(/`{1,3}[^`]*`{1,3}/gs, '')
    .replace(/\[(.+?)\]\(.+?\)/g, '$1')
    .replace(/^\s*[-*>|#]+\s*/gm, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .toLowerCase()
}

function titleFromMd(src: string): string {
  const m = src.match(/^#\s+(.+)$/m)
  return m ? m[1].trim() : ''
}

export function docsSearchIndexPlugin(): Plugin {
  return {
    name: 'docs-search-index',
    buildStart() {
      const sections = ['help', 'tech'] as const
      const entries: SearchEntry[] = []
      const base = fileURLToPath(new URL('./src/lib/docs', import.meta.url))

      for (const section of sections) {
        const dir = join(base, section)
        let files: string[]
        try {
          files = readdirSync(dir).filter((f: string) => f.endsWith('.md'))
        } catch {
          console.warn(`[docs-search-index] directory not found: ${dir}`)
          continue
        }
        for (const file of files) {
          const slug = file.replace(/\.md$/, '')
          const src = readFileSync(join(dir, file), 'utf8')
          entries.push({
            section,
            slug,
            title: titleFromMd(src),
            body: stripMd(src),
          })
        }
      }

      const out = fileURLToPath(
        new URL('./static/docs-search-index.json', import.meta.url)
      )
      writeFileSync(out, JSON.stringify(entries))
      console.log(`[docs-search-index] indexed ${entries.length} pages`)
    },
  }
}
