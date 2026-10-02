<script setup lang="ts">
import { ref, watch, onScopeDispose } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { Filter, X, SlidersHorizontal } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetDescription,
  SheetTrigger
} from '@/components/ui/sheet'
import SearchFilterFields from './SearchFilterFields.vue'
import { cn } from '@/lib/utils'
import type { SearchFilters as Filters, FacetData } from '@/api/client'

const props = withDefaults(
  defineProps<{
    filters: Filters
    facets: FacetData
    loading?: boolean
    activeCount?: number
    class?: string
  }>(),
  { loading: false, activeCount: 0 }
)
const emit = defineEmits<{ 'update:filters': [filters: Filters]; clear: [] }>()
function cloneFilters(filters: Filters): Filters {
  return {
    ...filters,
    companies: filters.companies && [...filters.companies],
    docTypes: filters.docTypes && [...filters.docTypes],
    tags: filters.tags && [...filters.tags]
  }
}
const draft = ref(cloneFilters(props.filters))
const mobileOpen = ref(false)
const desktop = useMediaQuery('(min-width: 1024px)')
let titleTimer: ReturnType<typeof setTimeout> | undefined
function cancelTitle() {
  clearTimeout(titleTimer)
  titleTimer = undefined
}
watch(
  () => props.filters,
  (filters) => {
    cancelTitle()
    draft.value = cloneFilters(filters)
  },
  { deep: true }
)
watch(desktop, (value) => {
  if (value) mobileOpen.value = false
})
onScopeDispose(cancelTitle)
function applyFilters() {
  const filters: Filters = {}
  for (const key of ['companies', 'docTypes', 'tags'] as const) {
    if (draft.value[key]?.length) filters[key] = [...draft.value[key]]
  }
  for (const key of ['dateFrom', 'dateTo'] as const) {
    if (draft.value[key]) filters[key] = draft.value[key]
  }
  if (draft.value.hasBarcode !== undefined) filters.hasBarcode = draft.value.hasBarcode
  if (draft.value.titleFilter?.trim()) filters.titleFilter = draft.value.titleFilter.trim()
  emit('update:filters', filters)
}
function update(patch: Partial<Filters>, debounce = false) {
  draft.value = { ...draft.value, ...patch }
  cancelTitle()
  if (debounce) titleTimer = setTimeout(applyFilters, 300)
  else applyFilters()
}
function clearAll() {
  cancelTitle()
  draft.value = {}
  emit('clear')
}
</script>
<template>
  <Sheet v-model:open="mobileOpen">
    <div class="lg:hidden">
      <SheetTrigger as-child
        ><Button
          type="button"
          variant="outline"
          size="sm"
          class="gap-2"
          aria-label="Open search filters"
          ><SlidersHorizontal :size="15" aria-hidden="true" />Filters<Badge
            v-if="activeCount"
            class="h-5 min-w-5 justify-center px-1"
            >{{ activeCount }}</Badge
          ></Button
        ></SheetTrigger
      >
    </div>
    <SheetContent
      side="left"
      class="flex w-[min(360px,calc(100vw-24px))] flex-col gap-0 p-0 sm:max-w-sm lg:hidden"
    >
      <div class="border-b px-5 pb-4 pt-5">
        <SheetTitle class="flex items-center gap-2 text-base"
          ><Filter :size="16" aria-hidden="true" />Search filters</SheetTitle
        ><SheetDescription class="mt-1 text-xs"
          >Narrow your search by company, dates, or document details.</SheetDescription
        >
      </div>
      <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <SearchFilterFields :filters="draft" :facets="facets" :loading="loading" @change="update" />
      </div>
      <div class="flex items-center gap-2 border-t p-4">
        <Button v-if="activeCount" type="button" variant="outline" size="sm" @click="clearAll"
          ><X :size="14" aria-hidden="true" />Clear all filters</Button
        ><Button type="button" size="sm" class="ml-auto" @click="mobileOpen = false"
          >Show results</Button
        >
      </div>
    </SheetContent>
  </Sheet>
  <aside :class="cn('hidden w-64 shrink-0 lg:block', props.class)" aria-label="Search filters">
    <div class="sticky top-24 space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="flex items-center gap-2 text-sm font-medium">
          <Filter :size="16" aria-hidden="true" />Filters<Badge
            v-if="activeCount"
            class="h-5 min-w-5 justify-center px-1"
            >{{ activeCount }}</Badge
          >
        </h2>
        <Button
          v-if="activeCount"
          type="button"
          variant="ghost"
          size="sm"
          class="h-7 text-xs"
          @click="clearAll"
          >Clear all</Button
        >
      </div>
      <div class="rounded-xl border bg-card p-4">
        <SearchFilterFields :filters="draft" :facets="facets" :loading="loading" @change="update" />
      </div>
    </div>
  </aside>
</template>
