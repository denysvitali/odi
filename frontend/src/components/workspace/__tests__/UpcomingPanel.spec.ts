import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UpcomingPanel from '../UpcomingPanel.vue'
import type { ReminderBucket } from '@/composables/useReminders'
const buckets: ReminderBucket[] = [
  {
    key: 'thisWeek',
    label: 'This week',
    reminders: Array.from({ length: 4 }, (_, i) => ({
      id: `sample-${i}`,
      title: `Sample ${i}`,
      dueDate: '2026-10-05',
      amountDue: 'CHF 20'
    }))
  },
  { key: 'thisMonth', label: 'This month', reminders: [] },
  {
    key: 'later',
    label: 'Later',
    reminders: [{ id: 'sample/encoded', title: 'Later sample', dueDate: '2026-12-01' }]
  }
]
const options = {
  global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } }
}

describe('upcoming workspace deadlines', () => {
  it('groups dates, omits empty groups, and retains amounts in expanded rows', () => {
    const wrapper = mount(UpcomingPanel, {
      ...options,
      props: { buckets, loading: false, error: null }
    })
    expect(wrapper.text()).toContain('Next 7 days')
    expect(wrapper.text()).toContain('Later')
    expect(wrapper.text()).not.toContain('Next 30 days')
    expect(wrapper.get('summary').text()).toBe('Show 1 more deadline')
    expect(wrapper.get('details').text()).toContain('CHF 20')
    expect(wrapper.find('a[href="/documents/sample%2Fencoded"]').exists()).toBe(true)
  })
  it('prioritizes loading and failure states, retries, and renders the empty state', async () => {
    const wrapper = mount(UpcomingPanel, {
      ...options,
      props: { buckets, loading: true, error: null }
    })
    expect(wrapper.get('[role="status"]').text()).toContain('Checking')
    expect(wrapper.find('.reminder-item').exists()).toBe(false)
    await wrapper.setProps({ loading: false, error: 'Unavailable' })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([[]])
    await wrapper.setProps({ error: null, buckets: [] })
    expect(wrapper.text()).toContain('Nothing coming up.')
  })
})
