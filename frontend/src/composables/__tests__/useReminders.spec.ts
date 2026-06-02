import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useReminders } from '@/composables/useReminders'

declare global {

  var _settings: { apiUrl: string; opensearchUrl: string }
}

beforeEach(() => {
  ;(window as unknown as { _settings: unknown })._settings = {
    apiUrl: 'http://api.test',
    opensearchUrl: ''
  }
  vi.restoreAllMocks()
})

function isoIn(days: number): string {
  return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString()
}

describe('useReminders', () => {
  it('loads reminders and echoes the window', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        days: 90,
        reminders: [
          { id: 'a', title: 'Invoice', dueDate: isoIn(3), docType: 'invoice', company: 'EWZ', amountDue: 'CHF 100' }
        ]
      })
    }) as unknown as typeof fetch

    const r = useReminders()
    await r.load()

    expect(r.reminders.value).toHaveLength(1)
    expect(r.days.value).toBe(90)
    expect(r.hasReminders.value).toBe(true)
    expect(r.total.value).toBe(1)
  })

  it('passes the days query param when provided', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ days: 30, reminders: [] })
    })
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const r = useReminders()
    await r.load(30)

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const calledUrl = String(fetchMock.mock.calls[0][0])
    expect(calledUrl).toContain('/reminders')
    expect(calledUrl).toContain('days=30')
    expect(r.days.value).toBe(30)
  })

  it('buckets reminders into this week / this month / later', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        days: 90,
        reminders: [
          { id: 'week', title: 'W', dueDate: isoIn(2) },
          { id: 'month', title: 'M', dueDate: isoIn(20) },
          { id: 'later', title: 'L', dueDate: isoIn(75) }
        ]
      })
    }) as unknown as typeof fetch

    const r = useReminders()
    await r.load()

    const byKey = Object.fromEntries(r.buckets.value.map((b) => [b.key, b.reminders.map((x) => x.id)]))
    expect(byKey.thisWeek).toEqual(['week'])
    expect(byKey.thisMonth).toEqual(['month'])
    expect(byKey.later).toEqual(['later'])
  })

  it('surfaces a user-facing error on failure', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Server Error',
      headers: new Headers()
    }) as unknown as typeof fetch

    const r = useReminders()
    await r.load()

    expect(r.error.value).toBeTruthy()
    expect(r.reminders.value).toEqual([])
  })
})
