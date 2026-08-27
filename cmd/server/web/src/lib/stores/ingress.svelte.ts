import type { IngressLogEntry, IngressRule, IngressSource } from '$lib/api'
import { ingressApi } from '$lib/api'
import { toastStore } from './toast.svelte'

// ---- Sources ----

let sources = $state<IngressSource[]>([])
let loadingSources = $state(false)

// ---- Log ----

let log = $state<IngressLogEntry[]>([])
let loadingLog = $state(false)
let logSourceId = $state<string | null>(null)

// ---- Rules ----

let rules = $state<IngressRule[]>([])
let loadingRules = $state(false)
let rulesSourceId = $state<string | null>(null)

export const ingressStore = {
  get sources() {
    return sources
  },
  get loadingSources() {
    return loadingSources
  },

  get log() {
    return log
  },
  get loadingLog() {
    return loadingLog
  },
  get logSourceId() {
    return logSourceId
  },

  get rules() {
    return rules
  },
  get loadingRules() {
    return loadingRules
  },
  get rulesSourceId() {
    return rulesSourceId
  },

  async loadSources() {
    loadingSources = true
    try {
      sources = (await ingressApi.list()) ?? []
    } catch {
      toastStore.show('无法加载导入来源', 'error')
    } finally {
      loadingSources = false
    }
  },

  async createSource(
    params: Parameters<typeof ingressApi.create>[0]
  ): Promise<IngressSource | null> {
    try {
      const src = await ingressApi.create(params)
      sources = [src, ...sources]
      return src
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '无法创建导入来源'
      toastStore.show(msg, 'error')
      return null
    }
  },

  async updateSource(
    id: string,
    params: Parameters<typeof ingressApi.update>[1]
  ): Promise<IngressSource | null> {
    try {
      const src = await ingressApi.update(id, params)
      sources = sources.map((s) => (s.id === id ? src : s))
      return src
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '无法更新导入来源'
      toastStore.show(msg, 'error')
      return null
    }
  },

  async deleteSource(id: string): Promise<boolean> {
    try {
      await ingressApi.delete(id)
      sources = sources.filter((s) => s.id !== id)
      toastStore.show('导入来源已删除')
      return true
    } catch {
      toastStore.show('无法删除导入来源', 'error')
      return false
    }
  },

  async toggleSource(id: string, enabled: boolean): Promise<void> {
    try {
      const src = await ingressApi.update(id, { enabled })
      sources = sources.map((s) => (s.id === id ? src : s))
    } catch {
      toastStore.show('无法更新导入来源', 'error')
    }
  },

  async pollSource(id: string): Promise<void> {
    try {
      await ingressApi.poll(id)
      toastStore.show('已开始扫描服务器目录')
    } catch {
      toastStore.show('无法开始扫描', 'error')
    }
  },

  // Log

  async loadLog(sourceId: string, status?: string): Promise<void> {
    loadingLog = true
    logSourceId = sourceId
    try {
      log = await ingressApi.getSourceLog(sourceId, status)
    } catch {
      toastStore.show('无法加载导入记录', 'error')
    } finally {
      loadingLog = false
    }
  },

  async retryLogEntry(entryId: string): Promise<void> {
    try {
      await ingressApi.retryLogEntry(entryId)
      toastStore.show('已重新加入导入队列')
      // Refresh log for current source
      if (logSourceId) await ingressStore.loadLog(logSourceId)
    } catch {
      toastStore.show('无法重试该文件', 'error')
    }
  },

  async deleteLogEntry(entryId: string): Promise<void> {
    try {
      await ingressApi.deleteLogEntry(entryId)
      log = log.filter((e) => e.id !== entryId)
    } catch {
      toastStore.show('无法删除导入记录', 'error')
    }
  },

  // Rules

  async loadRules(sourceId: string): Promise<void> {
    loadingRules = true
    rulesSourceId = sourceId
    try {
      rules = await ingressApi.listRules(sourceId)
    } catch {
      toastStore.show('无法加载整理规则', 'error')
    } finally {
      loadingRules = false
    }
  },

  async createRule(
    sourceId: string,
    params: Parameters<typeof ingressApi.createRule>[1]
  ): Promise<IngressRule | null> {
    try {
      const rule = await ingressApi.createRule(sourceId, params)
      rules = [...rules, rule].sort((a, b) => a.position - b.position)
      return rule
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '无法创建整理规则'
      toastStore.show(msg, 'error')
      return null
    }
  },

  async updateRule(
    sourceId: string,
    ruleId: string,
    params: Parameters<typeof ingressApi.updateRule>[2]
  ): Promise<void> {
    try {
      const rule = await ingressApi.updateRule(sourceId, ruleId, params)
      rules = rules.map((r) => (r.id === ruleId ? rule : r))
    } catch {
      toastStore.show('无法更新整理规则', 'error')
    }
  },

  async deleteRule(sourceId: string, ruleId: string): Promise<void> {
    try {
      await ingressApi.deleteRule(sourceId, ruleId)
      rules = rules.filter((r) => r.id !== ruleId)
    } catch {
      toastStore.show('无法删除整理规则', 'error')
    }
  },
}
