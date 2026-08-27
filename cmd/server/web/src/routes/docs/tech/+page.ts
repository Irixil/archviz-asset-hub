import { extractDescription, loadDoc } from '$lib/docs/loader'
import { findEntry } from '$lib/docs/nav'
import { error } from '@sveltejs/kit'
import type { PageLoad } from './$types'

export const prerender = true

export const load: PageLoad = async () => {
  const entry = findEntry('tech', 'index')
  if (!entry) throw error(404, 'Page not found')
  const raw = await loadDoc('tech', 'index')
  const description = extractDescription(raw)
  return { entry, raw, description }
}
