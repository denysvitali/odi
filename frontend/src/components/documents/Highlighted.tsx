import { Mark } from '@mantine/core'
import { parseHighlight } from '@/lib/highlight'

/** Render OpenSearch `<em>` highlights as text + <mark>, never as HTML. */
export function Highlighted({ text }: { text: string }) {
  return parseHighlight(text).map((seg, i) =>
    seg.match ? <Mark key={i}>{seg.text}</Mark> : <span key={i}>{seg.text}</span>
  )
}

const escapeRegex = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

/** Highlight case-insensitive occurrences of `needle` in plain text. */
export function FindHighlight({ text, needle }: { text: string; needle: string }) {
  const n = needle.trim()
  if (!n) return text
  return text
    .split(new RegExp(`(${escapeRegex(n)})`, 'gi'))
    .map((part, i) => (i % 2 ? <Mark key={i}>{part}</Mark> : part))
}

export function countMatches(text: string, needle: string) {
  const n = needle.trim()
  return n ? (text.match(new RegExp(escapeRegex(n), 'gi')) || []).length : 0
}
