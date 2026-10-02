<script setup lang="ts">
import { computed } from 'vue'
import { Building2, Calendar, QrCode, Type, ChevronDown } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import DocTypeFacet from './DocTypeFacet.vue'
import TagFacet from './TagFacet.vue'
import type { SearchFilters, FacetData, FacetBucket } from '@/api/client'

const props = defineProps<{ filters: SearchFilters; facets: FacetData; loading?: boolean }>()
const emit = defineEmits<{ change: [patch: Partial<SearchFilters>, debounce?: boolean] }>()
function includeSelected(buckets: FacetBucket[], selected: string[] = []) {
  return [
    ...buckets,
    ...selected
      .filter((key) => !buckets.some((bucket) => bucket.key === key))
      .map((key) => ({ key, doc_count: -1 }))
  ]
}
const companies = computed(() => includeSelected(props.facets.companies, props.filters.companies))
const docTypes = computed(() =>
  includeSelected(props.facets.docTypes || [], props.filters.docTypes)
)
const tags = computed(() => includeSelected(props.facets.tags || [], props.filters.tags))
function toggleCompany(key: string) {
  const selected = props.filters.companies || []
  emit('change', {
    companies: selected.includes(key)
      ? selected.filter((value) => value !== key)
      : [...selected, key]
  })
}
</script>
<template>
  <div class="space-y-5" :aria-busy="loading">
    <details v-if="companies.length" open class="group border-b border-border pb-4">
      <summary
        class="flex cursor-pointer list-none items-center justify-between text-sm font-medium"
      >
        <span class="flex items-center gap-2"
          ><Building2 :size="15" class="text-muted-foreground" aria-hidden="true" />Company</span
        ><ChevronDown
          :size="14"
          class="transition-transform group-open:rotate-180"
          aria-hidden="true"
        />
      </summary>
      <div class="mt-3 space-y-1">
        <label
          v-for="bucket in companies"
          :key="bucket.key"
          class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-secondary"
          ><input
            type="checkbox"
            :checked="filters.companies?.includes(bucket.key) || false"
            class="h-4 w-4 accent-primary"
            @change="toggleCompany(bucket.key)"
          /><span class="min-w-0 flex-1 truncate">{{ bucket.key }}</span
          ><span class="text-muted-foreground">{{
            bucket.doc_count < 0 ? 'Selected' : bucket.doc_count
          }}</span></label
        >
      </div>
    </details>
    <details open class="group border-b border-border pb-4">
      <summary
        class="flex cursor-pointer list-none items-center justify-between text-sm font-medium"
      >
        <span class="flex items-center gap-2"
          ><Calendar :size="15" class="text-muted-foreground" aria-hidden="true" />Date Range</span
        ><ChevronDown
          :size="14"
          class="transition-transform group-open:rotate-180"
          aria-hidden="true"
        />
      </summary>
      <div class="mt-3 grid gap-3">
        <label class="grid gap-1 text-xs text-muted-foreground"
          >From<Input
            :model-value="filters.dateFrom || ''"
            type="date"
            class="h-9"
            :max="filters.dateTo || undefined"
            @update:model-value="emit('change', { dateFrom: $event })" /></label
        ><label class="grid gap-1 text-xs text-muted-foreground"
          >To<Input
            :model-value="filters.dateTo || ''"
            type="date"
            class="h-9"
            :min="filters.dateFrom || undefined"
            @update:model-value="emit('change', { dateTo: $event })"
        /></label>
      </div>
    </details>
    <details open class="group border-b border-border pb-4">
      <summary
        class="flex cursor-pointer list-none items-center justify-between text-sm font-medium"
      >
        <span class="flex items-center gap-2"
          ><QrCode :size="15" class="text-muted-foreground" aria-hidden="true" />Barcode</span
        ><ChevronDown
          :size="14"
          class="transition-transform group-open:rotate-180"
          aria-hidden="true"
        />
      </summary>
      <div class="mt-3">
        <label class="grid gap-1.5 text-xs text-muted-foreground"
          >Barcode presence<select
            class="h-9 w-full rounded-md border border-input bg-background px-2 text-sm text-foreground"
            :value="filters.hasBarcode === undefined ? 'any' : String(filters.hasBarcode)"
            @change="
              emit('change', {
                hasBarcode:
                  ($event.target as HTMLSelectElement).value === 'any'
                    ? undefined
                    : ($event.target as HTMLSelectElement).value === 'true'
              })
            "
          >
            <option value="any">Any</option>
            <option value="true">With barcode</option>
            <option value="false">Without barcode</option>
          </select></label
        >
        <p v-if="facets.barcodeCount > 0" class="mt-2 text-xs text-muted-foreground">
          {{ facets.barcodeCount }} with barcode ·
          {{ Math.max(0, facets.totalHits - facets.barcodeCount) }} without
        </p>
      </div>
    </details>
    <details open class="group">
      <summary
        class="flex cursor-pointer list-none items-center justify-between text-sm font-medium"
      >
        <span class="flex items-center gap-2"
          ><Type :size="15" class="text-muted-foreground" aria-hidden="true" />Title</span
        ><ChevronDown
          :size="14"
          class="transition-transform group-open:rotate-180"
          aria-hidden="true"
        />
      </summary>
      <label class="mt-3 grid gap-1 text-xs text-muted-foreground"
        >Document title<Input
          :model-value="filters.titleFilter || ''"
          type="text"
          placeholder="Filter by title…"
          class="h-9"
          @update:model-value="emit('change', { titleFilter: $event }, true)"
      /></label>
    </details>
    <DocTypeFacet
      v-if="docTypes.length"
      :buckets="docTypes"
      :selected="filters.docTypes || []"
      @update="emit('change', { docTypes: $event })"
    />
    <TagFacet
      v-if="tags.length"
      :buckets="tags"
      :selected="filters.tags || []"
      @update="emit('change', { tags: $event })"
    />
    <p v-if="loading" role="status" class="text-xs text-muted-foreground">Updating filters…</p>
  </div>
</template>
