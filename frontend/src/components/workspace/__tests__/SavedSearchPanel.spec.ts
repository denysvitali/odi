import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import SavedSearchPanel from '../SavedSearchPanel.vue'
const search = { id: 'sample', name: 'Invoices', term: '', filters: { docTypes: ['invoice'] } }
afterEach(() => {
  document.body.innerHTML = ''
})

describe('saved search management', () => {
  it('focuses the name, saves a rename, and reports success', async () => {
    const rename = vi.fn(() => true)
    const wrapper = mount(SavedSearchPanel, {
      attachTo: document.body,
      props: { searches: [search], error: '', rename }
    })
    await wrapper.get('[aria-label="Rename saved search Invoices"]').trigger('click')
    expect(document.activeElement).toBe(wrapper.get('input').element)
    await wrapper.get('input').setValue('Bills')
    await wrapper.get('form').trigger('submit')
    expect(rename).toHaveBeenCalledWith('sample', 'Bills')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.get('[role="status"]').text()).toBe('Saved search renamed.')
    wrapper.unmount()
  })
  it('keeps the editor open on failure and cancels with Escape without saving', async () => {
    const rename = vi.fn(() => false)
    const wrapper = mount(SavedSearchPanel, { props: { searches: [search], error: '', rename } })
    await wrapper.get('[aria-label="Rename saved search Invoices"]').trigger('click')
    await wrapper.get('input').setValue('Bills')
    await wrapper.get('form').trigger('submit')
    await wrapper.setProps({ error: 'Changes were not saved.' })
    expect(wrapper.get('input').element.value).toBe('Bills')
    expect(wrapper.get('[role="alert"]').text()).toContain('not saved')
    await wrapper.get('input').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('form').exists()).toBe(false)
    expect(rename).toHaveBeenCalledTimes(1)
  })
  it('selects the original saved query and removes by identifier', async () => {
    const wrapper = mount(SavedSearchPanel, {
      props: { searches: [search], error: '', rename: () => true }
    })
    await wrapper.get('.saved-search-row button').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[search]])
    await wrapper.get('[aria-label="Remove saved search Invoices"]').trigger('click')
    expect(wrapper.emitted('remove')).toEqual([['sample']])
  })
})
