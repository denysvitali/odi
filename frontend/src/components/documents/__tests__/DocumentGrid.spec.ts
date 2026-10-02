import { mount } from '@vue/test-utils'
import { beforeEach, describe, it, expect } from 'vitest'
import DocumentGrid from '../DocumentGrid.vue'

const documents = [
  {
    _id: 'synthetic-b',
    _source: { title: 'Zulu document', text: 'Synthetic text', date: '2026-01-01' }
  },
  {
    _id: 'synthetic-a',
    _source: { title: 'Alpha document', text: 'Synthetic text', date: '2026-03-01' }
  },
  { _id: 'synthetic-c', _source: { title: 'Undated document', text: 'Synthetic text' } }
]
const mountGrid = (selectable = false) =>
  mount(DocumentGrid, {
    props: { documents, selectable, selectedIds: new Set(['synthetic-b']) },
    global: {
      stubs: {
        DocumentCard: {
          props: ['document'],
          template: '<div role="button" tabindex="0">{{ document._source.title }}</div>'
        }
      }
    }
  })
beforeEach(() => localStorage.clear())
describe('document view controls', () => {
  it('sorts loaded rows without mutating documents and puts undated records last', async () => {
    const grid = mountGrid()
    await grid.get('select').setValue('title')
    expect(grid.findAll('.document-list-title').map((row) => row.text())).toEqual([
      'Alpha document',
      'Undated document',
      'Zulu document'
    ])
    await grid.get('select').setValue('newest')
    expect(grid.findAll('.document-list-title').map((row) => row.text())).toEqual([
      'Alpha document',
      'Zulu document',
      'Undated document'
    ])
    expect(documents[0]._id).toBe('synthetic-b')
    grid.unmount()
  })
  it('opens documents normally and emits selection events in selection mode', async () => {
    const grid = mountGrid()
    await grid.get('.document-list-main').trigger('click')
    expect(grid.emitted('selectDocument')?.[0]).toEqual([documents[0]])
    await grid.setProps({ selectable: true })
    await grid.get('.document-list-main').trigger('click')
    expect(grid.emitted('toggleSelect')?.[0]).toEqual([documents[0]])
    expect(grid.get('.document-list-main').attributes('aria-pressed')).toBe('true')
    grid.unmount()
  })
  it('renders escaped search excerpts in the list and keeps metadata accessible', async () => {
    const grid = mountGrid()
    await grid.setProps({
      searchTerm: 'invoice',
      documents: [
        {
          _id: 'excerpt',
          _source: { title: 'Invoice', text: '', docType: 'invoice', date: '2026-01-01' },
          highlight: { text: ['Found <em>invoice</em> <img src=x onerror=alert(1)>'] }
        }
      ]
    })
    expect(grid.get('mark').text()).toBe('invoice')
    expect(grid.find('img').exists()).toBe(false)
    expect(grid.text()).toContain('<img src=x onerror=alert(1)>')
    expect(grid.find('[aria-label="Type: invoice"]').exists()).toBe(true)
    expect(grid.find('[aria-label^="Document date:"]').exists()).toBe(true)
    grid.unmount()
  })
  it('remembers the view and lets keyboard arrows focus cards without intercepting controls', async () => {
    const grid = mountGrid()
    await grid.get('[aria-label="Grid view"]').trigger('click')
    expect(localStorage.getItem('odi-document-view')).toBe('grid')
    const cards = grid.findAll('.grid > [role="button"]')
    await cards[0].trigger('keydown', { key: 'ArrowRight' })
    // No grid-wide Enter interception: the card owns its activation.
    await grid.get('[aria-label="List view"]').trigger('keydown', { key: 'Enter' })
    expect(grid.emitted('selectDocument')).toBeUndefined()
    grid.unmount()
    const restored = mountGrid()
    expect(restored.get('[aria-label="Grid view"]').attributes('aria-pressed')).toBe('true')
    restored.unmount()
  })
})
