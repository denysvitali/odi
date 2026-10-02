import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import FavoritesView from '../FavoritesView.vue'

const saved = ref<string[]>([])
vi.mock('@/api/client', () => ({ api: { getDocumentDetails: vi.fn() } }))
vi.mock('@/composables/useFavorites', () => ({
  useFavorites: () => ({
    list: saved,
    clear: () => {
      saved.value = []
    }
  })
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/lib/config', () => ({ getOpensearchUrl: () => '' }))
const fetchDetails = vi.mocked(api.getDocumentDetails)
const render = () =>
  mount(FavoritesView, {
    global: {
      stubs: {
        PageContainer: { template: '<main><slot /></main>' },
        DocumentDetailSheet: true,
        DocumentGrid: {
          props: ['documents', 'loading', 'emptyTitle'],
          template:
            '<section><span v-for="doc in documents" :key="doc._id">{{ doc._source.title }}</span><p v-if="!loading && !documents.length">{{ emptyTitle }}</p></section>'
        }
      }
    }
  })
beforeEach(() => {
  fetchDetails.mockReset()
  saved.value = []
})

describe('favorites view', () => {
  it('shows local empty state without making archive requests', async () => {
    const view = render()
    await flushPromises()
    expect(view.text()).toContain('No favorites yet')
    expect(fetchDetails).not.toHaveBeenCalled()
    view.unmount()
  })

  it('shows partial results and retries unavailable favorites without deleting saved stars', async () => {
    saved.value = ['older-document', 'missing-document']
    fetchDetails.mockImplementation(async (id) => {
      if (id === 'missing-document') throw new Error('synthetic missing')
      return { id, title: 'Older document', text: '' }
    })
    const view = render()
    expect(view.get('[role="status"]').text()).toContain('2 remaining')
    await flushPromises()
    expect(view.text()).toContain('Older document')
    expect(view.get('[role="alert"]').text()).toContain('1 favorite could not be loaded')
    expect(saved.value).toHaveLength(2)
    fetchDetails.mockResolvedValue({
      id: 'missing-document',
      title: 'Recovered document',
      text: ''
    })
    await view.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(view.find('[role="alert"]').exists()).toBe(false)
    expect(view.text()).toContain('Recovered document')
    expect(fetchDetails).toHaveBeenCalledTimes(3)
    view.unmount()
  })
})
