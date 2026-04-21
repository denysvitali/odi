/**
 * Safely render search-highlight text from OpenSearch.
 *
 * OpenSearch returns text with `<em>...</em>` tags marking matches. We cannot
 * trust arbitrary HTML inside that text, so we split on those specific tags,
 * escape all other content, and rebuild markup ourselves.
 */

const EM_OPEN = '<em>'
const EM_CLOSE = '</em>'

export interface HighlightSegment {
  text: string
  match: boolean
}

export function parseHighlight(input: string): HighlightSegment[] {
  const segments: HighlightSegment[] = []
  let remaining = input
  while (remaining.length > 0) {
    const openIdx = remaining.indexOf(EM_OPEN)
    if (openIdx === -1) {
      segments.push({ text: remaining, match: false })
      break
    }
    if (openIdx > 0) {
      segments.push({ text: remaining.slice(0, openIdx), match: false })
    }
    const afterOpen = remaining.slice(openIdx + EM_OPEN.length)
    const closeIdx = afterOpen.indexOf(EM_CLOSE)
    if (closeIdx === -1) {
      segments.push({ text: afterOpen, match: true })
      break
    }
    segments.push({ text: afterOpen.slice(0, closeIdx), match: true })
    remaining = afterOpen.slice(closeIdx + EM_CLOSE.length)
  }
  return segments
}

export function stripHighlight(input: string): string {
  return parseHighlight(input).map((s) => s.text).join('')
}
