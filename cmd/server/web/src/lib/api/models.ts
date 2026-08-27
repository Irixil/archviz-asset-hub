export interface MenuItem {
  id: string | null
  label: string
  url?: string
  color?: string
  count?: number
}

export type IngressSourceType =
  | 'email_api'
  | 'imap'
  | 'sftp'
  | 'dav'
  | 's3'
  | 'gdrive'
  | 'canva'

export type FieldType =
  | 'text'
  | 'number'
  | 'date'
  | 'boolean'
  | 'select'
  | 'url'
export type FieldScope = 'asset' | 'project'

export interface FieldFilter {
  key: string
  op: 'eq' | 'lt' | 'lte' | 'gt' | 'gte' | 'contains' | 'starts_with'
  value: string
}
