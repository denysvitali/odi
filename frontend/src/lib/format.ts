import { getLocale, DEFAULT_CURRENCY } from './constants'

export function formatDate(input: string | Date | undefined | null): string {
  if (!input) return ''
  const date = typeof input === 'string' ? new Date(input) : input
  if (Number.isNaN(date.getTime())) return String(input)
  return new Intl.DateTimeFormat(getLocale(), {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  }).format(date)
}

export function formatDateTime(input: string | Date | undefined | null): string {
  if (!input) return ''
  const date = typeof input === 'string' ? new Date(input) : input
  if (Number.isNaN(date.getTime())) return String(input)
  return new Intl.DateTimeFormat(getLocale(), {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

export function formatCurrency(amount: number | undefined, currency?: string): string | null {
  if (amount === undefined || amount === null) return null
  return new Intl.NumberFormat(getLocale(), {
    style: 'currency',
    currency: currency || DEFAULT_CURRENCY
  }).format(amount)
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat(getLocale()).format(n)
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

export function formatRelative(date: Date): string {
  const diffMs = Date.now() - date.getTime()
  const sec = Math.floor(diffMs / 1000)
  if (sec < 5) return 'just now'
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day}d ago`
  return formatDate(date)
}
