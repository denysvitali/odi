<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Search, FileText, Home, Upload, Sun, Moon, Keyboard, Trash2, Shield, Loader2 } from 'lucide-vue-next'
import { useTheme } from '@/composables/useTheme'
import { useDocumentStore } from '@/stores/documents'
import { useSearch } from '@/composables/useSearch'
import HighlightedText from '@/components/documents/HighlightedText.vue'
import { api } from '@/api/client'
import { formatDate } from '@/lib/format'
import { extractCompanyFromText } from '@/lib/documentMetadata'
import type { Document } from '@/types/documents'

interface Action {
  id: string
  label: string
  hint?: string
  icon: unknown
  perform: () => void
  keywords?: string
}

interface Props {
  open: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  'show-shortcuts': []
  'select-document': [doc: Document]
}>()

const router = useRouter()
const { toggleTheme, isDark } = useTheme()
const store = useDocumentStore()

const query = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const selected = ref(0)

// Document search
const {
  results: searchResults,
  loading: searchLoading,
  hasSearched,
  debouncedSearch,
  clear: clearSearch
} = useSearch({ debounceMs: 200, pageSize: 5 })

const actions = computed<Action[]>(() => [
  {
    id: 'go-home',
    label: 'Go to Home',
    hint: 'g h',
    icon: Home,
    keywords: 'home search',
    perform: () => router.push('/')
  },
  {
    id: 'go-documents',
    label: 'Browse all documents',
    hint: 'g d',
    icon: FileText,
    keywords: 'documents browse list',
    perform: () => router.push('/documents')
  },
  {
    id: 'go-upload',
    label: 'Upload documents',
    hint: 'g u',
    icon: Upload,
    keywords: 'upload scan add',
    perform: () => router.push('/upload')
  },
  {
    id: 'go-admin',
    label: 'Open admin',
    icon: Shield,
    keywords: 'admin reindex b2 import',
    perform: () => router.push('/admin')
  },
  {
    id: 'toggle-theme',
    label: isDark.value ? 'Switch to light mode' : 'Switch to dark mode',
    icon: isDark.value ? Sun : Moon,
    keywords: 'theme dark light',
    perform: () => toggleTheme()
  },
  {
    id: 'shortcuts',
    label: 'Show keyboard shortcuts',
    hint: '?',
    icon: Keyboard,
    keywords: 'help keys shortcuts',
    perform: () => emit('show-shortcuts')
  },
  {
    id: 'clear-history',
    label: 'Clear recent searches',
    icon: Trash2,
    keywords: 'recent history clear',
    perform: () => store.clearRecentSearches()
  }
])

const recentSearches = computed<Action[]>(() =>
  store.recentSearches.slice(0, 5).map((term) => ({
    id: `search-${term}`,
    label: `Search "${term}"`,
    icon: Search,
    keywords: term,
    perform: () => router.push({ path: '/', query: { q: term } })
  }))
)

const filtered = computed<Action[]>(() => {
  const all = [...actions.value, ...recentSearches.value]
  const q = query.value.trim().toLowerCase()
  if (!q) return all
  return all.filter((a) =>
    (a.label + ' ' + (a.keywords || '')).toLowerCase().includes(q)
  )
})

const docResults = computed(() => searchResults.value.slice(0, 5))
const totalItems = computed(() => filtered.value.length + docResults.value.length)

// Trigger document search as user types
watch(query, (q) => {
  if (q.trim()) {
    debouncedSearch(q)
  } else {
    clearSearch()
  }
})

// Keep selected in bounds when result count changes
watch(docResults, () => {
  if (selected.value >= totalItems.value) {
    selected.value = Math.max(0, totalItems.value - 1)
  }
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      query.value = ''
      selected.value = 0
      nextTick(() => inputRef.value?.focus())
    } else {
      clearSearch()
    }
  }
)

watch(filtered, () => {
  selected.value = 0
})

const close = () => emit('update:open', false)

const run = (a: Action) => {
  a.perform()
  close()
}

const openDocument = (doc: Document) => {
  emit('select-document', doc)
  close()
}

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    selected.value = Math.min(selected.value + 1, totalItems.value - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    selected.value = Math.max(selected.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (selected.value < filtered.value.length) {
      const a = filtered.value[selected.value]
      if (a) run(a)
      else if (query.value.trim()) {
        router.push({ path: '/', query: { q: query.value.trim() } })
        close()
      }
    } else {
      const docIndex = selected.value - filtered.value.length
      const doc = docResults.value[docIndex]
      if (doc) openDocument(doc)
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}

const thumbnailUrl = (id: string) => api.thumbnailUrl(id)

const getCompanyName = (doc: Document): string =>
  doc._source.company?.name || extractCompanyFromText(doc._source.text || '')

const getSnippet = (doc: Document): string => {
  if (doc.highlight?.text?.[0]) return doc.highlight.text[0]
  const text = doc._source.text || ''
  return text.length > 150 ? text.slice(0, 150) + '…' : text
}
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-200"
      leave-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="props.open"
        class="fixed inset-0 z-[70] flex items-start justify-center bg-background/70 p-4 pt-[15vh] backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        @click.self="close"
      >
        <div class="w-full max-w-xl overflow-hidden rounded-2xl border bg-card shadow-2xl">
          <div class="flex items-center border-b px-4">
            <Search class="h-4 w-4 text-muted-foreground" aria-hidden="true" />
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              placeholder="Search documents or type a command…"
              aria-label="Command palette input"
              class="h-12 w-full bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground"
              @keydown="onKeydown"
            />
            <kbd class="hidden rounded border bg-background px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground sm:inline-block">
              Esc
            </kbd>
          </div>
          <ul class="max-h-[50vh] overflow-auto p-2" role="listbox">
            <!-- Actions -->
            <li
              v-for="(a, i) in filtered"
              :key="a.id"
              :aria-selected="i === selected"
              :class="[
                'flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm',
                i === selected ? 'bg-primary/10 text-foreground' : 'text-muted-foreground'
              ]"
              role="option"
              @mouseenter="selected = i"
              @click="run(a)"
            >
              <component :is="a.icon" class="h-4 w-4" aria-hidden="true" />
              <span class="flex-1">{{ a.label }}</span>
              <kbd v-if="a.hint" class="rounded border bg-background px-1.5 py-0.5 text-[10px]">
                {{ a.hint }}
              </kbd>
            </li>

            <!-- Documents section -->
            <template v-if="query.trim()">
              <li
                class="mx-1 mt-1 flex items-center gap-2 border-t px-2 pb-1 pt-2 text-xs font-medium text-muted-foreground"
                role="separator"
              >
                <FileText class="h-3 w-3" aria-hidden="true" />
                Documents
              </li>

              <li
                v-for="(doc, di) in docResults"
                :key="doc._id"
                :aria-selected="filtered.length + di === selected"
                :class="[
                  'flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm',
                  filtered.length + di === selected ? 'bg-primary/10 text-foreground' : 'text-muted-foreground'
                ]"
                role="option"
                @mouseenter="selected = filtered.length + di"
                @click="openDocument(doc)"
              >
                <div class="h-10 w-8 shrink-0 overflow-hidden rounded bg-muted">
                  <img
                    :src="thumbnailUrl(doc._id)"
                    :alt="doc._source.title || 'Document thumbnail'"
                    class="h-full w-full object-cover"
                    loading="lazy"
                    decoding="async"
                  />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm">
                    <HighlightedText :text="getSnippet(doc)" />
                  </div>
                  <div
                    v-if="getCompanyName(doc) || doc._source.date"
                    class="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground"
                  >
                    <span v-if="getCompanyName(doc)" class="max-w-[140px] truncate">{{ getCompanyName(doc) }}</span>
                    <span v-if="getCompanyName(doc) && doc._source.date" aria-hidden="true">&middot;</span>
                    <span v-if="doc._source.date" class="shrink-0">{{ formatDate(doc._source.date) }}</span>
                  </div>
                </div>
              </li>

              <li v-if="searchLoading" class="flex items-center gap-3 px-3 py-3 text-sm text-muted-foreground">
                <Loader2 class="h-4 w-4 animate-spin" aria-hidden="true" />
                <span>Searching…</span>
              </li>

              <li
                v-else-if="hasSearched && docResults.length === 0"
                class="px-3 py-4 text-center text-sm text-muted-foreground"
              >
                No documents found
              </li>
            </template>

            <!-- Empty state when no query and no actions -->
            <li
              v-if="filtered.length === 0 && !query.trim()"
              class="px-3 py-6 text-center text-sm text-muted-foreground"
            >
              Type a command or search term…
            </li>
          </ul>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
