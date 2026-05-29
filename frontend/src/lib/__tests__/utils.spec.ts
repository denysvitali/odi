import { describe, it, expect } from 'vitest'
import { cn, errorMessage } from '@/lib/utils'

describe('cn', () => {
  it('merges and dedupes tailwind classes', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4')
    expect(cn('text-sm', false && 'hidden', 'font-bold')).toBe('text-sm font-bold')
  })
})

describe('errorMessage', () => {
  it('returns the message of an Error', () => {
    expect(errorMessage(new Error('boom'), 'fallback')).toBe('boom')
  })

  it('returns the message of an Error subclass', () => {
    class ApiError extends Error {}
    expect(errorMessage(new ApiError('api down'), 'fallback')).toBe('api down')
  })

  it('falls back for non-Error values', () => {
    expect(errorMessage('a string', 'fallback')).toBe('fallback')
    expect(errorMessage(null, 'fallback')).toBe('fallback')
    expect(errorMessage(undefined, 'fallback')).toBe('fallback')
    expect(errorMessage({ message: 'fake' }, 'fallback')).toBe('fallback')
  })
})
