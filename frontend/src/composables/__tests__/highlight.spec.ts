import { describe, it, expect } from 'vitest'
import { parseHighlight, stripHighlight } from '@/lib/highlight'

describe('parseHighlight', () => {
  it('returns a single non-match segment when there are no tags', () => {
    expect(parseHighlight('hello world')).toEqual([
      { text: 'hello world', match: false }
    ])
  })

  it('splits on <em> tags', () => {
    expect(parseHighlight('hi <em>world</em>!')).toEqual([
      { text: 'hi ', match: false },
      { text: 'world', match: true },
      { text: '!', match: false }
    ])
  })

  it('does not interpret other HTML as tags (XSS-safe)', () => {
    const input = '<script>alert(1)</script> <em>ok</em>'
    const segs = parseHighlight(input)
    // The <script> portion is returned as text, not interpreted.
    expect(segs[0]).toEqual({ text: '<script>alert(1)</script> ', match: false })
    expect(segs[1]).toEqual({ text: 'ok', match: true })
  })

  it('handles unclosed <em>', () => {
    const segs = parseHighlight('a <em>b c')
    expect(segs[segs.length - 1]).toEqual({ text: 'b c', match: true })
  })
})

describe('stripHighlight', () => {
  it('removes em markers but keeps inner text', () => {
    expect(stripHighlight('a <em>b</em> c')).toBe('a b c')
  })
})
