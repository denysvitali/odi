import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowMount, flushPromises } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import DocumentsView from '../DocumentsView.vue'
import { api } from '@/api/client'

let route: { params: { id?: string } }
const replace = vi.fn()
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace, push: vi.fn() })
}))
vi.mock('@/composables/useInfiniteScroll', () => ({
  useInfiniteScroll: () => ({ targetRef: null })
}))

const mountView = () =>
  shallowMount(DocumentsView, {
    global: {
      stubs: {
        PageContainer: { template: '<div><slot /></div>' },
        Button: { template: '<button><slot /></button>' },
        Input: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        }
      }
    }
  })
beforeEach(() => {
  route = reactive({ params: {} })
  window._settings = { apiUrl: 'https://api.test/api/v1', opensearchUrl: '' }
  vi.spyOn(api, 'listDocuments').mockResolvedValue({
    hits: { hits: [], total: { value: 0, relation: 'eq' } }
  })
  replace.mockClear()
})
afterEach(() => vi.restoreAllMocks())

const button = (wrapper: ReturnType<typeof mountView>, label: string) =>
  wrapper.findAll('button').find((b) => b.text() === label)!

describe('document browsing', () => {
  it('fetches date filters on Apply and removes them on Clear', async () => {
    const wrapper = mountView()
    await flushPromises()
    await button(wrapper, 'Filters').trigger('click')
    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('2024-01-01')
    await inputs[1].setValue('2024-01-31')
    await button(wrapper, 'Apply').trigger('click')
    await flushPromises()
    expect(api.listDocuments).toHaveBeenLastCalledWith({
      size: 12,
      dateFrom: '2024-01-01',
      dateTo: '2024-01-31'
    })
    await button(wrapper, 'Filters').trigger('click')
    await button(wrapper, 'Clear').trigger('click')
    await flushPromises()
    expect(api.listDocuments).toHaveBeenLastCalledWith({
      size: 12,
      dateFrom: undefined,
      dateTo: undefined
    })
    wrapper.unmount()
  })

  it('resolves deep links outside the first page and route changes', async () => {
    route.params.id = 'older_1'
    vi.spyOn(api, 'getDocumentDetails').mockImplementation(async (id) => ({
      id,
      text: 'synthetic',
      title: id
    }))
    const wrapper = mountView()
    await flushPromises()
    const sheet = wrapper.findComponent({ name: 'DocumentDetailSheet' })
    expect(sheet.props('document')._id).toBe('older_1')
    expect(sheet.props('open')).toBe(true)
    route.params.id = 'different_1'
    await nextTick()
    await flushPromises()
    expect(sheet.props('document')._id).toBe('different_1')
    vi.mocked(api.getDocumentDetails).mockRejectedValueOnce(new Error('Document unavailable'))
    route.params.id = 'missing_1'
    await nextTick()
    await flushPromises()
    expect(sheet.props('open')).toBe(false)
    expect(sheet.props('document')).toBeNull()
    wrapper.unmount()
  })

  it('shows a failed deep link instead of silently ignoring it', async () => {
    route.params.id = 'missing_1'
    vi.spyOn(api, 'getDocumentDetails').mockRejectedValue(new Error('Document unavailable'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe('Document unavailable')
    wrapper.unmount()
  })
})
