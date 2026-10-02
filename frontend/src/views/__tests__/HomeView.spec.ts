import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowMount, flushPromises } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import { createPinia } from 'pinia'
import HomeView from '../HomeView.vue'
import { api } from '@/api/client'

let route: { query: Record<string, string> }
const replace = vi.fn()
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace }) }))
const mountView = () =>
  shallowMount(HomeView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        PageContainer: { template: '<div><slot /></div>' },
        Button: { template: '<button><slot /></button>' }
      }
    }
  })
beforeEach(() => {
  route = reactive({ query: {} })
  window._settings = { apiUrl: 'https://api.test/api/v1', opensearchUrl: '' }
  vi.spyOn(api, 'search').mockResolvedValue({
    hits: { hits: [], total: { value: 0, relation: 'eq' } }
  })
  replace.mockClear()
})
afterEach(() => vi.restoreAllMocks())

describe('search page navigation', () => {
  it('restores filter-only bookmarks and notices filter changes with the same query', async () => {
    route.query = { q: '', tags: 'tax', docTypes: 'invoice' }
    const wrapper = mountView()
    await flushPromises()
    expect(api.search).toHaveBeenLastCalledWith({
      searchTerm: '',
      size: 100,
      filters: { docTypes: ['invoice'], tags: ['tax'] }
    })
    route.query = { q: '', tags: 'archive', docTypes: 'invoice' }
    await nextTick()
    await flushPromises()
    expect(api.search).toHaveBeenLastCalledWith({
      searchTerm: '',
      size: 100,
      filters: { docTypes: ['invoice'], tags: ['archive'] }
    })
    wrapper.unmount()
  })

  it('renders search failures with a retry action', async () => {
    route.query = { q: 'synthetic' }
    vi.mocked(api.search).mockRejectedValue(new Error('Search temporarily unavailable'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('Search temporarily unavailable')
    expect(wrapper.findAll('button').some((button) => button.text() === 'Try again')).toBe(true)
    wrapper.unmount()
  })
})
