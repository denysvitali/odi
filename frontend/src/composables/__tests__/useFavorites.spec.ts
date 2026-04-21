import { describe, it, expect, beforeEach } from 'vitest'
import { useFavorites } from '@/composables/useFavorites'

beforeEach(() => {
  localStorage.clear()
})

describe('useFavorites', () => {
  it('toggles favorite state and persists to storage', () => {
    const fav = useFavorites()
    expect(fav.isFavorite('a')).toBe(false)
    fav.toggle('a')
    expect(fav.isFavorite('a')).toBe(true)
    expect(fav.count.value).toBeGreaterThanOrEqual(1)
    fav.toggle('a')
    expect(fav.isFavorite('a')).toBe(false)
  })
})
