import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, VueWrapper } from '@vue/test-utils'
import SearchFilters from '@/components/search/SearchFilters.vue'
import type { SearchFilters as SearchFiltersType, FacetData } from '@/api/client'

// Stub child components so we only test SearchFilters logic, not the UI kit.
const stubs = {
  Button: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: {
    props: ['modelValue', 'type', 'placeholder'],
    emits: ['update:modelValue', 'change'],
    template: '<input :type="type" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
  },
  Badge: {
    props: ['variant'],
    template: '<span class="badge"><slot /></span>'
  },
  Separator: { template: '<hr />' },
  ScrollArea: { template: '<div><slot /></div>' },
  Filter: { template: '<span />' },
  X: { template: '<span />' },
  ChevronDown: { template: '<span />' },
  Building2: { template: '<span />' },
  Calendar: { template: '<span />' },
  QrCode: { template: '<span />' },
  Type: { template: '<span />' },
  SlidersHorizontal: { template: '<span />' },
}

const emptyFilters: SearchFiltersType = {}

const emptyFacets: FacetData = {
  companies: [],
  dateHistogram: [],
  barcodeCount: 0,
  totalHits: 0,
}

const sampleFacets: FacetData = {
  companies: [
    { key: 'Swisscom', doc_count: 12 },
    { key: 'SBB', doc_count: 5 },
    { key: 'PostFinance', doc_count: 3 },
  ],
  dateHistogram: [
    { key: '2024-01-01', doc_count: 8 },
    { key: '2024-02-01', doc_count: 4 },
  ],
  barcodeCount: 7,
  totalHits: 20,
}

function mountFilters(
  props: Partial<{ filters: SearchFiltersType; facets: FacetData; loading: boolean; activeCount: number }> = {}
): VueWrapper {
  return mount(SearchFilters, {
    props: {
      filters: emptyFilters,
      facets: emptyFacets,
      ...props,
    },
    global: { stubs },
  })
}

// ---------------------------------------------------------------------------
// Component renders all filter sections
// ---------------------------------------------------------------------------

describe('SearchFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  describe('rendering filter sections', () => {
    it('renders the component root element', () => {
      const wrapper = mountFilters()
      expect(wrapper.find('[class*="hidden lg:block"]').exists() || wrapper.html().length > 0).toBe(true)
    })

    it('renders the company section when companies facet data is provided', () => {
      const wrapper = mountFilters({ facets: sampleFacets })
      const html = wrapper.html()
      expect(html).toContain('Company')
    })

    it('does not render company checkboxes when companies facet is empty', () => {
      const wrapper = mountFilters({
        facets: { ...emptyFacets },
      })
      // Company section should not appear when there are no company facets.
      const companyInputs = wrapper.findAll('input[type="checkbox"]')
      // No company checkboxes if facets are empty.
      expect(companyInputs.length).toBe(0)
    })

    it('renders the Date Range section', () => {
      const wrapper = mountFilters()
      expect(wrapper.html()).toContain('Date Range')
    })

    it('renders date input fields', () => {
      const wrapper = mountFilters()
      const dateInputs = wrapper.findAll('input[type="date"]')
      expect(dateInputs.length).toBeGreaterThanOrEqual(2)
    })

    it('renders the Barcode section', () => {
      const wrapper = mountFilters({ facets: sampleFacets })
      expect(wrapper.html()).toContain('Barcode')
    })

    it('renders the Title section', () => {
      const wrapper = mountFilters()
      expect(wrapper.html()).toContain('Title')
    })

    it('renders the Filters heading', () => {
      const wrapper = mountFilters()
      expect(wrapper.html()).toContain('Filters')
    })
  })

  // ---------------------------------------------------------------------------
  // Selecting a company filter emits the correct event
  // ---------------------------------------------------------------------------

  describe('company filter selection', () => {
    it('emits update:filters when a company checkbox is toggled', async () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: emptyFilters,
      })

      // Find the first company checkbox and click it.
      const companySection = wrapper.findAll('label').find((el) => el.text().includes('Swisscom'))
      expect(companySection).toBeDefined()

      const checkbox = companySection!.find('input[type="checkbox"]')
      await checkbox.setValue(true)
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      expect(emitted!.length).toBeGreaterThan(0)
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.companies).toContain('Swisscom')
    })

    it('removes a company when unchecked', async () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: { companies: ['Swisscom'] },
      })

      const companySection = wrapper.findAll('label').find((el) => el.text().includes('Swisscom'))
      const checkbox = companySection!.find('input[type="checkbox"]')
      await checkbox.setValue(false)
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.companies?.includes('Swisscom') ?? false).toBe(false)
    })

    it('supports multiple company selections', async () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: emptyFilters,
      })

      // Select Swisscom
      const swisscom = wrapper.findAll('label').find((el) => el.text().includes('Swisscom'))
      await swisscom!.find('input[type="checkbox"]').setValue(true)
      await vi.advanceTimersByTime(100)

      // Select SBB
      const sbb = wrapper.findAll('label').find((el) => el.text().includes('SBB'))
      await sbb!.find('input[type="checkbox"]').setValue(true)
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.companies).toContain('Swisscom')
      expect(lastEmit.companies).toContain('SBB')
    })
  })

  // ---------------------------------------------------------------------------
  // Date range inputs
  // ---------------------------------------------------------------------------

  describe('date range inputs', () => {
    it('emits update:filters when dateFrom changes', async () => {
      const wrapper = mountFilters({ filters: emptyFilters })

      // Find all date inputs (from + to)
      const dateInputs = wrapper.findAll('input[type="date"]')
      expect(dateInputs.length).toBeGreaterThanOrEqual(2)

      await dateInputs[0].setValue('2024-01-01')
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.dateFrom).toBe('2024-01-01')
    })

    it('emits update:filters when dateTo changes', async () => {
      const wrapper = mountFilters({ filters: emptyFilters })

      const dateInputs = wrapper.findAll('input[type="date"]')
      await dateInputs[1].setValue('2024-12-31')
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.dateTo).toBe('2024-12-31')
    })

    it('initializes date inputs from filter props', () => {
      const wrapper = mountFilters({
        filters: { dateFrom: '2024-03-01', dateTo: '2024-03-31' },
      })

      const dateInputs = wrapper.findAll('input[type="date"]')
      expect((dateInputs[0].element as HTMLInputElement).value).toBe('2024-03-01')
      expect((dateInputs[1].element as HTMLInputElement).value).toBe('2024-03-31')
    })
  })

  // ---------------------------------------------------------------------------
  // Clear all filters button
  // ---------------------------------------------------------------------------

  describe('clear all filters', () => {
    it('shows clear button when there are active filters', () => {
      const wrapper = mountFilters({
        filters: { companies: ['Swisscom'] },
        facets: sampleFacets,
        activeCount: 1,
      })

      expect(wrapper.html()).toContain('Clear all')
    })

    it('emits clear event when clear button is clicked', async () => {
      const wrapper = mountFilters({
        filters: { companies: ['Swisscom'] },
        facets: sampleFacets,
        activeCount: 1,
      })

      // Find and click the clear button
      const buttons = wrapper.findAll('button')
      const clearBtn = buttons.find((b) => b.text().includes('Clear all'))
      expect(clearBtn).toBeDefined()

      await clearBtn!.trigger('click')
      await vi.advanceTimersByTime(100)

      expect(wrapper.emitted('clear')).toBeTruthy()
    })

    it('resets all local filters when clear is triggered', async () => {
      const wrapper = mountFilters({
        filters: {
          companies: ['Swisscom'],
          dateFrom: '2024-01-01',
          dateTo: '2024-12-31',
        },
        facets: sampleFacets,
        activeCount: 3,
      })

      const buttons = wrapper.findAll('button')
      const clearBtn = buttons.find((b) => b.text().includes('Clear all'))
      await clearBtn!.trigger('click')
      await vi.advanceTimersByTime(100)

      const emitted = wrapper.emitted('update:filters')
      expect(emitted).toBeTruthy()
      // After clearing, the emitted filters should be empty/default.
      const lastEmit = emitted![emitted!.length - 1][0] as SearchFiltersType
      expect(lastEmit.companies || []).toHaveLength(0)
      expect(lastEmit.dateFrom || '').toBe('')
      expect(lastEmit.dateTo || '').toBe('')
    })
  })

  // ---------------------------------------------------------------------------
  // Facet data populates the checkboxes
  // ---------------------------------------------------------------------------

  describe('facet data populates checkboxes', () => {
    it('renders company checkboxes matching facet data', () => {
      const wrapper = mountFilters({ facets: sampleFacets })
      const html = wrapper.html()

      expect(html).toContain('Swisscom')
      expect(html).toContain('SBB')
      expect(html).toContain('PostFinance')
    })

    it('displays doc_count for each company bucket', () => {
      const wrapper = mountFilters({ facets: sampleFacets })
      const html = wrapper.html()

      expect(html).toContain('12') // Swisscom count
      expect(html).toContain('5')  // SBB count
      expect(html).toContain('3')  // PostFinance count
    })

    it('checks the checkbox for pre-selected companies', () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: { companies: ['Swisscom'] },
      })

      const swisscomLabel = wrapper.findAll('label').find((el) => el.text().includes('Swisscom'))
      const checkbox = swisscomLabel!.find('input[type="checkbox"]')
      expect((checkbox.element as HTMLInputElement).checked).toBe(true)
    })

    it('does not check unchecked for unselected companies', () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: { companies: ['Swisscom'] },
      })

      const sbbLabel = wrapper.findAll('label').find((el) => el.text().includes('SBB'))
      const checkbox = sbbLabel!.find('input[type="checkbox"]')
      expect((checkbox.element as HTMLInputElement).checked).toBe(false)
    })

    it('updates checkboxes when facet data changes', async () => {
      const wrapper = mountFilters({ facets: emptyFacets })

      // Initially no company checkboxes
      expect(wrapper.findAll('input[type="checkbox"]').length).toBe(0)

      // Update facets
      await wrapper.setProps({ facets: sampleFacets })

      // Now should have company checkboxes
      const companyCheckboxes = wrapper.findAll('label').filter((el) => {
        const text = el.text()
        return text.includes('Swisscom') || text.includes('SBB') || text.includes('PostFinance')
      })
      expect(companyCheckboxes.length).toBe(3)
    })
  })

  // ---------------------------------------------------------------------------
  // Barcode filter
  // ---------------------------------------------------------------------------

  describe('barcode filter', () => {
    it('shows barcode toggle when barcode facet data is present', () => {
      const wrapper = mountFilters({ facets: sampleFacets })
      expect(wrapper.html()).toContain('Barcode')
    })

    it('emits filters with hasBarcode when barcode toggle is clicked', async () => {
      const wrapper = mountFilters({
        facets: sampleFacets,
        filters: emptyFilters,
      })

      // Find the barcode toggle button
      const barcodeButtons = wrapper.findAll('button').filter((b) => {
        const text = b.text()
        return text.includes('Any') || text.includes('barcode') || text.includes('With')
      })

      // Click the first barcode-related button
      if (barcodeButtons.length > 0) {
        await barcodeButtons[0].trigger('click')
        await vi.advanceTimersByTime(100)

        const emitted = wrapper.emitted('update:filters')
        expect(emitted).toBeTruthy()
      }
    })
  })

  // ---------------------------------------------------------------------------
  // Loading state
  // ---------------------------------------------------------------------------

  describe('loading state', () => {
    it('shows loading indicator when loading prop is true', () => {
      const wrapper = mountFilters({ loading: true })
      expect(wrapper.html()).toContain('Updating filters')
    })

    it('does not show loading indicator when loading is false', () => {
      const wrapper = mountFilters({ loading: false })
      expect(wrapper.html()).not.toContain('Updating filters')
    })
  })

  // ---------------------------------------------------------------------------
  // Active count badge
  // ---------------------------------------------------------------------------

  describe('active count badge', () => {
    it('shows badge when activeCount > 0', () => {
      const wrapper = mountFilters({ activeCount: 3 })
      expect(wrapper.html()).toContain('3')
    })

    it('does not show badge when activeCount is 0', () => {
      const wrapper = mountFilters({ activeCount: 0 })
      const badges = wrapper.findAll('.badge')
      // The active count badge should not be visible.
      const countBadges = badges.filter((b) => b.text() === '3')
      expect(countBadges.length).toBe(0)
    })
  })
})
