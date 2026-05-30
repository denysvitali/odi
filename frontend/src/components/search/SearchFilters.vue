<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  Filter,
  X,
  ChevronDown,
  Building2,
  Calendar,
  QrCode,
  Type,
  SlidersHorizontal,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import DocTypeFacet from './DocTypeFacet.vue'
import TagFacet from './TagFacet.vue'
import type { SearchFilters as SearchFiltersType, FacetData } from '@/api/client'

interface Props {
  filters: SearchFiltersType
  facets: FacetData
  loading?: boolean
  activeCount?: number
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  activeCount: 0,
})

const emit = defineEmits<{
  'update:filters': [filters: SearchFiltersType]
  clear: []
}>()

// Collapsible sections
const expandedSections = ref<Set<string>>(new Set(['companies', 'dates', 'barcode', 'title']))

const toggleSection = (section: string) => {
  if (expandedSections.value.has(section)) {
    expandedSections.value.delete(section)
  } else {
    expandedSections.value.add(section)
  }
}

const isSectionExpanded = (section: string) => expandedSections.value.has(section)

// Local filter state for batching updates
const localFilters = ref<SearchFiltersType>({ ...props.filters })

watch(
  () => props.filters,
  (newFilters) => {
    localFilters.value = { ...newFilters }
  },
  { deep: true }
)

// Company filter
const selectedCompanies = ref<string[]>(props.filters.companies || [])

watch(
  () => props.filters.companies,
  (companies) => {
    selectedCompanies.value = companies || []
  },
  { deep: true }
)

const toggleCompany = (company: string) => {
  const idx = selectedCompanies.value.indexOf(company)
  if (idx >= 0) {
    selectedCompanies.value.splice(idx, 1)
  } else {
    selectedCompanies.value.push(company)
  }
  emitFilters()
}

const isCompanySelected = (company: string) => selectedCompanies.value.includes(company)

// Document type filter
const selectedDocTypes = ref<string[]>(props.filters.docTypes || [])

watch(
  () => props.filters.docTypes,
  (docTypes) => {
    selectedDocTypes.value = docTypes || []
  },
  { deep: true }
)

const updateDocTypes = (keys: string[]) => {
  selectedDocTypes.value = keys
  emitFilters()
}

// Tag filter
const selectedTags = ref<string[]>(props.filters.tags || [])

watch(
  () => props.filters.tags,
  (tags) => {
    selectedTags.value = tags || []
  },
  { deep: true }
)

const updateTags = (keys: string[]) => {
  selectedTags.value = keys
  emitFilters()
}

// Date range filter
const dateFrom = ref(props.filters.dateFrom || '')
const dateTo = ref(props.filters.dateTo || '')

watch(
  () => props.filters.dateFrom,
  (val) => {
    dateFrom.value = val || ''
  }
)

watch(
  () => props.filters.dateTo,
  (val) => {
    dateTo.value = val || ''
  }
)

watch([dateFrom, dateTo], () => {
  emitFilters()
})

// Barcode filter
const hasBarcode = ref<boolean | undefined>(props.filters.hasBarcode)

watch(
  () => props.filters.hasBarcode,
  (val) => {
    hasBarcode.value = val
  }
)

const toggleBarcode = () => {
  if (hasBarcode.value === undefined) {
    hasBarcode.value = true
  } else if (hasBarcode.value === true) {
    hasBarcode.value = false
  } else {
    hasBarcode.value = undefined
  }
  emitFilters()
}

// Title filter
const titleFilter = ref(props.filters.titleFilter || '')

watch(
  () => props.filters.titleFilter,
  (val) => {
    titleFilter.value = val || ''
  }
)

let titleDebounce: ReturnType<typeof setTimeout> | null = null
watch(titleFilter, () => {
  if (titleDebounce) clearTimeout(titleDebounce)
  titleDebounce = setTimeout(() => {
    emitFilters()
  }, 300)
})

// Emit aggregated filters
const emitFilters = () => {
  const filters: SearchFiltersType = {}

  if (selectedCompanies.value.length > 0) {
    filters.companies = [...selectedCompanies.value]
  }
  if (dateFrom.value) {
    filters.dateFrom = dateFrom.value
  }
  if (dateTo.value) {
    filters.dateTo = dateTo.value
  }
  if (hasBarcode.value !== undefined) {
    filters.hasBarcode = hasBarcode.value
  }
  if (titleFilter.value.trim()) {
    filters.titleFilter = titleFilter.value.trim()
  }
  if (selectedDocTypes.value.length > 0) {
    filters.docTypes = [...selectedDocTypes.value]
  }
  if (selectedTags.value.length > 0) {
    filters.tags = [...selectedTags.value]
  }

  emit('update:filters', filters)
}

const clearAll = () => {
  selectedCompanies.value = []
  dateFrom.value = ''
  dateTo.value = ''
  hasBarcode.value = undefined
  titleFilter.value = ''
  selectedDocTypes.value = []
  selectedTags.value = []
  emit('clear')
}

// Mobile drawer state
const isMobileOpen = ref(false)

const toggleMobile = () => {
  isMobileOpen.value = !isMobileOpen.value
}

// Computed facet display
const hasCompanies = computed(() => props.facets.companies.length > 0)
const hasDateData = computed(() => props.facets.dateHistogram.length > 0)
const hasBarcodeData = computed(() => props.facets.barcodeCount > 0)
const hasDocTypes = computed(() => (props.facets.docTypes?.length ?? 0) > 0)
const hasTags = computed(() => (props.facets.tags?.length ?? 0) > 0)

const barcodeLabel = computed(() => {
  if (hasBarcode.value === true) return 'With barcode'
  if (hasBarcode.value === false) return 'Without barcode'
  return 'Any'
})

const totalWithoutBarcode = computed(() => props.facets.totalHits - props.facets.barcodeCount)
</script>

<template>
  <!-- Mobile toggle button -->
  <div class="lg:hidden">
    <Button
      variant="outline"
      size="sm"
      class="gap-2"
      @click="toggleMobile"
    >
      <SlidersHorizontal class="h-4 w-4" />
      Filters
      <Badge v-if="activeCount > 0" variant="default" class="ml-1 h-5 min-w-5 justify-center px-1">
        {{ activeCount }}
      </Badge>
    </Button>
  </div>

  <!-- Mobile drawer overlay -->
  <Teleport to="body">
    <div
      v-if="isMobileOpen"
      class="fixed inset-0 z-40 bg-black/50 lg:hidden"
      @click="isMobileOpen = false"
    />
    <div
      :class="cn(
        'fixed inset-y-0 left-0 z-50 w-80 transform bg-background shadow-xl transition-transform duration-300 lg:hidden',
        isMobileOpen ? 'translate-x-0' : '-translate-x-full'
      )"
    >
      <div class="flex h-full flex-col">
        <div class="flex items-center justify-between border-b px-4 py-3">
          <div class="flex items-center gap-2">
            <Filter class="h-4 w-4" />
            <span class="font-medium">Filters</span>
            <Badge v-if="activeCount > 0" variant="default" class="h-5 min-w-5 justify-center px-1">
              {{ activeCount }}
            </Badge>
          </div>
          <Button variant="ghost" size="icon" @click="isMobileOpen = false">
            <X class="h-4 w-4" />
          </Button>
        </div>
        <ScrollArea class="flex-1 px-4 py-3">
          <div class="space-y-4">
            <!-- Company filter section -->
            <div v-if="hasCompanies">
              <button
                type="button"
                class="flex w-full items-center justify-between py-2 text-sm font-medium"
                @click="toggleSection('companies')"
              >
                <span class="flex items-center gap-2">
                  <Building2 class="h-4 w-4 text-muted-foreground" />
                  Company
                </span>
                <ChevronDown
                  :class="cn(
                    'h-4 w-4 text-muted-foreground transition-transform',
                    isSectionExpanded('companies') && 'rotate-180'
                  )"
                />
              </button>
              <div v-if="isSectionExpanded('companies')" class="mt-1 space-y-1 pl-6">
                <label
                  v-for="bucket in facets.companies"
                  :key="bucket.key"
                  class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                >
                  <input
                    type="checkbox"
                    :checked="isCompanySelected(bucket.key)"
                    class="h-4 w-4 rounded border-input"
                    @change="toggleCompany(bucket.key)"
                  />
                  <span class="flex-1 truncate">{{ bucket.key }}</span>
                  <span class="text-xs text-muted-foreground">{{ bucket.doc_count }}</span>
                </label>
              </div>
            </div>

            <Separator v-if="hasCompanies && (hasDateData || hasBarcodeData || true)" />

            <!-- Date range section -->
            <div>
              <button
                type="button"
                class="flex w-full items-center justify-between py-2 text-sm font-medium"
                @click="toggleSection('dates')"
              >
                <span class="flex items-center gap-2">
                  <Calendar class="h-4 w-4 text-muted-foreground" />
                  Date Range
                </span>
                <ChevronDown
                  :class="cn(
                    'h-4 w-4 text-muted-foreground transition-transform',
                    isSectionExpanded('dates') && 'rotate-180'
                  )"
                />
              </button>
              <div v-if="isSectionExpanded('dates')" class="mt-1 space-y-2 pl-6">
                <div class="space-y-1">
                  <label class="text-xs text-muted-foreground">From</label>
                  <Input
                    v-model="dateFrom"
                    type="date"
                    class="h-9"
                  />
                </div>
                <div class="space-y-1">
                  <label class="text-xs text-muted-foreground">To</label>
                  <Input
                    v-model="dateTo"
                    type="date"
                    class="h-9"
                  />
                </div>
              </div>
            </div>

            <Separator v-if="hasBarcodeData || true" />

            <!-- Barcode section -->
            <div>
              <button
                type="button"
                class="flex w-full items-center justify-between py-2 text-sm font-medium"
                @click="toggleSection('barcode')"
              >
                <span class="flex items-center gap-2">
                  <QrCode class="h-4 w-4 text-muted-foreground" />
                  Barcode
                </span>
                <ChevronDown
                  :class="cn(
                    'h-4 w-4 text-muted-foreground transition-transform',
                    isSectionExpanded('barcode') && 'rotate-180'
                  )"
                />
              </button>
              <div v-if="isSectionExpanded('barcode')" class="mt-1 space-y-2 pl-6">
                <button
                  type="button"
                  :class="cn(
                    'flex w-full items-center justify-between rounded-md px-3 py-2 text-sm transition-colors',
                    hasBarcode !== undefined
                      ? 'bg-primary/10 text-primary'
                      : 'hover:bg-accent'
                  )"
                  @click="toggleBarcode"
                >
                  <span>{{ barcodeLabel }}</span>
                  <span v-if="hasBarcode !== undefined" class="text-xs text-muted-foreground">
                    {{ hasBarcode ? facets.barcodeCount : totalWithoutBarcode }}
                  </span>
                </button>
                <div v-if="hasBarcodeData" class="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{{ facets.barcodeCount }} with barcode</span>
                  <span>{{ totalWithoutBarcode }} without</span>
                </div>
              </div>
            </div>

            <Separator />

            <!-- Title section -->
            <div>
              <button
                type="button"
                class="flex w-full items-center justify-between py-2 text-sm font-medium"
                @click="toggleSection('title')"
              >
                <span class="flex items-center gap-2">
                  <Type class="h-4 w-4 text-muted-foreground" />
                  Title
                </span>
                <ChevronDown
                  :class="cn(
                    'h-4 w-4 text-muted-foreground transition-transform',
                    isSectionExpanded('title') && 'rotate-180'
                  )"
                />
              </button>
              <div v-if="isSectionExpanded('title')" class="mt-1 pl-6">
                <Input
                  v-model="titleFilter"
                  type="text"
                  placeholder="Filter by title..."
                  class="h-9"
                />
              </div>
            </div>

            <Separator v-if="hasDocTypes" />
            <DocTypeFacet
              v-if="hasDocTypes"
              :buckets="facets.docTypes ?? []"
              :selected="selectedDocTypes"
              @update="updateDocTypes"
            />

            <Separator v-if="hasTags" />
            <TagFacet
              v-if="hasTags"
              :buckets="facets.tags ?? []"
              :selected="selectedTags"
              @update="updateTags"
            />
          </div>
        </ScrollArea>

        <!-- Clear all button -->
        <div v-if="activeCount > 0" class="border-t px-4 py-3">
          <Button variant="outline" size="sm" class="w-full" @click="clearAll">
            <X class="mr-2 h-4 w-4" />
            Clear all filters
          </Button>
        </div>
      </div>
    </div>
  </Teleport>

  <!-- Desktop sidebar -->
  <div
    :class="cn(
      'hidden lg:block w-64 shrink-0',
      props.class
    )"
  >
    <div class="sticky top-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2 text-sm font-medium">
          <Filter class="h-4 w-4" />
          Filters
          <Badge v-if="activeCount > 0" variant="default" class="h-5 min-w-5 justify-center px-1">
            {{ activeCount }}
          </Badge>
        </div>
        <Button
          v-if="activeCount > 0"
          variant="ghost"
          size="sm"
          class="h-7 text-xs"
          @click="clearAll"
        >
          Clear all
        </Button>
      </div>

      <div class="space-y-4 rounded-xl border bg-card p-4">
        <!-- Company filter section -->
        <div v-if="hasCompanies">
          <button
            type="button"
            class="flex w-full items-center justify-between py-2 text-sm font-medium"
            @click="toggleSection('companies')"
          >
            <span class="flex items-center gap-2">
              <Building2 class="h-4 w-4 text-muted-foreground" />
              Company
            </span>
            <ChevronDown
              :class="cn(
                'h-4 w-4 text-muted-foreground transition-transform',
                isSectionExpanded('companies') && 'rotate-180'
              )"
            />
          </button>
          <div v-if="isSectionExpanded('companies')" class="mt-1 space-y-1 pl-6">
            <label
              v-for="bucket in facets.companies"
              :key="bucket.key"
              class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
            >
              <input
                type="checkbox"
                :checked="isCompanySelected(bucket.key)"
                class="h-4 w-4 rounded border-input"
                @change="toggleCompany(bucket.key)"
              />
              <span class="flex-1 truncate">{{ bucket.key }}</span>
              <span class="text-xs text-muted-foreground">{{ bucket.doc_count }}</span>
            </label>
          </div>
        </div>

        <Separator v-if="hasCompanies" />

        <!-- Date range section -->
        <div>
          <button
            type="button"
            class="flex w-full items-center justify-between py-2 text-sm font-medium"
            @click="toggleSection('dates')"
          >
            <span class="flex items-center gap-2">
              <Calendar class="h-4 w-4 text-muted-foreground" />
              Date Range
            </span>
            <ChevronDown
              :class="cn(
                'h-4 w-4 text-muted-foreground transition-transform',
                isSectionExpanded('dates') && 'rotate-180'
              )"
            />
          </button>
          <div v-if="isSectionExpanded('dates')" class="mt-1 space-y-2 pl-6">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">From</label>
              <Input
                v-model="dateFrom"
                type="date"
                class="h-9"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">To</label>
              <Input
                v-model="dateTo"
                type="date"
                class="h-9"
              />
            </div>
          </div>
        </div>

        <Separator />

        <!-- Barcode section -->
        <div>
          <button
            type="button"
            class="flex w-full items-center justify-between py-2 text-sm font-medium"
            @click="toggleSection('barcode')"
          >
            <span class="flex items-center gap-2">
              <QrCode class="h-4 w-4 text-muted-foreground" />
              Barcode
            </span>
            <ChevronDown
              :class="cn(
                'h-4 w-4 text-muted-foreground transition-transform',
                isSectionExpanded('barcode') && 'rotate-180'
              )"
            />
          </button>
          <div v-if="isSectionExpanded('barcode')" class="mt-1 space-y-2 pl-6">
            <button
              type="button"
              :class="cn(
                'flex w-full items-center justify-between rounded-md px-3 py-2 text-sm transition-colors',
                hasBarcode !== undefined
                  ? 'bg-primary/10 text-primary'
                  : 'hover:bg-accent'
              )"
              @click="toggleBarcode"
            >
              <span>{{ barcodeLabel }}</span>
              <span v-if="hasBarcode !== undefined" class="text-xs text-muted-foreground">
                {{ hasBarcode ? facets.barcodeCount : totalWithoutBarcode }}
              </span>
            </button>
            <div v-if="hasBarcodeData" class="flex items-center justify-between text-xs text-muted-foreground">
              <span>{{ facets.barcodeCount }} with barcode</span>
              <span>{{ totalWithoutBarcode }} without</span>
            </div>
          </div>
        </div>

        <Separator />

        <!-- Title section -->
        <div>
          <button
            type="button"
            class="flex w-full items-center justify-between py-2 text-sm font-medium"
            @click="toggleSection('title')"
          >
            <span class="flex items-center gap-2">
              <Type class="h-4 w-4 text-muted-foreground" />
              Title
            </span>
            <ChevronDown
              :class="cn(
                'h-4 w-4 text-muted-foreground transition-transform',
                isSectionExpanded('title') && 'rotate-180'
              )"
            />
          </button>
          <div v-if="isSectionExpanded('title')" class="mt-1 pl-6">
            <Input
              v-model="titleFilter"
              type="text"
              placeholder="Filter by title..."
              class="h-9"
            />
          </div>
        </div>

        <Separator v-if="hasDocTypes" />
        <DocTypeFacet
          v-if="hasDocTypes"
          :buckets="facets.docTypes ?? []"
          :selected="selectedDocTypes"
          @update="updateDocTypes"
        />

        <Separator v-if="hasTags" />
        <TagFacet
          v-if="hasTags"
          :buckets="facets.tags ?? []"
          :selected="selectedTags"
          @update="updateTags"
        />
      </div>

      <!-- Loading indicator -->
      <div v-if="loading" class="flex items-center gap-2 text-xs text-muted-foreground">
        <div class="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
        Updating filters...
      </div>
    </div>
  </div>
</template>
