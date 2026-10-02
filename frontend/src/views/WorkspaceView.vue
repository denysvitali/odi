<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowRight,
  Search,
  Files,
  Star,
  Bookmark,
  CalendarDays,
  FileText,
  Receipt,
  Landmark,
  Heart,
  FileCheck,
  X,
  Plus,
  Clock,
  CheckCircle2,
  Sparkles
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PageContainer from '@/components/layout/PageContainer.vue'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import SearchFilters from '@/components/search/SearchFilters.vue'
import { useDocuments } from '@/composables/useDocuments'
import { useReminders } from '@/composables/useReminders'
import { useFavorites } from '@/composables/useFavorites'
import { useSavedSearches } from '@/composables/useSavedSearches'
import { useArchiveSearch } from '@/composables/useArchiveSearch'
import { useFacets } from '@/composables/useFacets'
import { useDocumentStore } from '@/stores/documents'
import { archiveQuery, parseArchiveQuery, hasArchiveSearch } from '@/lib/archiveSearch'
import { formatNumber } from '@/lib/format'
import { getOpensearchUrl } from '@/lib/config'
import type { SearchFilters as Filters } from '@/api/client'
import type { Document } from '@/types/documents'

const route = useRoute()
const router = useRouter()
const store = useDocumentStore()
const recent = useDocuments({ initialPageSize: 6 })
const deadlines = useReminders()
const { count: favoriteCount } = useFavorites()
const { searches: savedSearches, save, remove, error: savedError } = useSavedSearches()
const { results, total, loading, loadingMore, error, scrollId, search, loadMore } =
  useArchiveSearch()
const term = ref('')
const activeTerm = ref('')
const filters = ref<Filters>({})
const {
  facets,
  loading: facetsLoading,
  error: facetsError,
  fetchFacets
} = useFacets(activeTerm, filters)
const searching = computed(() => hasArchiveSearch(activeTerm.value, filters.value))
const activeCount = computed(() =>
  Object.values(filters.value).reduce(
    (n, value) => n + (Array.isArray(value) ? value.length : 1),
    0
  )
)
const saving = ref(false)
const saveName = ref('')
const savedMessage = ref('')
const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)
const opensearchUrl = computed(() => getOpensearchUrl())
const today = new Intl.DateTimeFormat(undefined, {
  weekday: 'short',
  month: 'short',
  day: 'numeric'
}).format(new Date())
const collections = [
  { label: 'Invoices', type: 'invoice', hint: 'Bills & payments', icon: Receipt },
  { label: 'Contracts', type: 'contract', hint: 'Agreements & policies', icon: FileCheck },
  { label: 'Banking', type: 'bank', hint: 'Statements & accounts', icon: Landmark },
  { label: 'Medical', type: 'medical', hint: 'Health & records', icon: Heart }
]
function navigateSearch(value = term.value, nextFilters = filters.value) {
  if (value.trim()) store.addRecentSearch(value.trim())
  return router.push({ path: '/', query: archiveQuery(value, nextFilters) })
}
function openDocument(doc: Document) {
  selectedDocument.value = doc
  sheetOpen.value = true
}
function saveCurrent() {
  if (save(saveName.value, activeTerm.value, filters.value)) {
    saving.value = false
    savedMessage.value = 'Search saved on this device.'
  }
}
function startSave() {
  saveName.value = activeTerm.value || 'Filtered documents'
  savedMessage.value = ''
  saving.value = !saving.value
}
function dateParts(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return { month: 'Due', day: '—' }
  return {
    month: date.toLocaleDateString(undefined, { month: 'short', timeZone: 'UTC' }),
    day: date.toLocaleDateString(undefined, { day: 'numeric', timeZone: 'UTC' })
  }
}
watch(
  () => route.query,
  (query) => {
    const parsed = parseArchiveQuery(query)
    term.value = parsed.term
    activeTerm.value = parsed.term
    filters.value = parsed.filters
    saving.value = false
    savedMessage.value = ''
    void search(parsed.term, parsed.filters)
  },
  { immediate: true }
)
onMounted(() => {
  store.loadRecentSearches()
  void recent.loadDocuments()
  void deadlines.load(90)
})
</script>
<template>
  <PageContainer>
    <div class="workspace-heading">
      <div>
        <p class="eyebrow">YOUR PERSONAL ARCHIVE</p>
        <h1>{{ searching ? 'Find what matters.' : 'A place for every paper.' }}</h1>
        <p>
          {{
            searching
              ? 'Search the full text, narrow the details, find your document.'
              : 'Less searching. More finding. Your archive, at a glance.'
          }}
        </p>
      </div>
      <span class="today-label"><CalendarDays :size="14" aria-hidden="true" />{{ today }}</span>
    </div>
    <section :class="searching ? '' : 'archive-hero'" aria-label="Search your archive">
      <template v-if="!searching"
        ><p class="eyebrow">A CLEARER DESK. A CLEARER HEAD.</p>
        <h2>Your paperwork.<br /><span>Finally in its place.</span></h2>
        <p>
          Find the invoice, the agreement, the detail you need.<br class="hidden sm:block" />
          Every word in your archive is searchable.
        </p>
        <div class="hero-paper" aria-hidden="true">
          <FileText :size="23" /><span /><span /><span /><span /></div
      ></template>
      <div class="workspace-search" :class="{ 'mt-0! max-w-none!': searching }">
        <form class="search-form" role="search" @submit.prevent="navigateSearch()">
          <Search :size="18" class="shrink-0 text-muted-foreground" aria-hidden="true" /><input
            v-model="term"
            type="search"
            aria-label="Search documents"
            data-global-search
            placeholder="Search a name, a phrase, a little detail…"
            autocomplete="off"
          /><Button type="submit" size="sm"
            >Search<ArrowRight :size="14" aria-hidden="true"
          /></Button>
        </form>
        <p class="search-hint">
          Try an exact phrase in quotes, or combine terms with AND, OR, NOT.
        </p>
      </div>
      <div v-if="!searching && store.recentSearches.length" class="recent-searches">
        <Clock :size="12" aria-hidden="true" /><span>Recent</span
        ><button
          v-for="query in store.recentSearches.slice(0, 4)"
          :key="query"
          type="button"
          @click="navigateSearch(query, {})"
        >
          {{ query }}</button
        ><button
          type="button"
          aria-label="Clear recent searches"
          @click="store.clearRecentSearches()"
        >
          <X :size="12" />
        </button>
      </div>
    </section>
    <template v-if="!searching">
      <div class="archive-stats">
        <RouterLink to="/documents" class="stat-card"
          ><span class="stat-icon"><Files :size="18" aria-hidden="true" /></span>
          <div>
            <div class="stat-number">
              {{
                recent.loading.value
                  ? '…'
                  : recent.error.value
                    ? '—'
                    : formatNumber(recent.total.value)
              }}
            </div>
            <p class="stat-label">Indexed documents</p>
            <p class="stat-detail">All in one place</p>
          </div></RouterLink
        >
        <RouterLink to="/favorites" class="stat-card"
          ><span class="stat-icon"><Star :size="18" aria-hidden="true" /></span>
          <div>
            <div class="stat-number">{{ favoriteCount }}</div>
            <p class="stat-label">Favorites</p>
            <p class="stat-detail">Your go-to documents</p>
          </div></RouterLink
        >
        <a href="#saved-searches" class="stat-card"
          ><span class="stat-icon"><Bookmark :size="18" aria-hidden="true" /></span>
          <div>
            <div class="stat-number">{{ savedSearches.length }}</div>
            <p class="stat-label">Saved searches</p>
            <p class="stat-detail">Pick up where you left off</p>
          </div></a
        >
        <a href="#upcoming" class="stat-card"
          ><span class="stat-icon"><CalendarDays :size="18" aria-hidden="true" /></span>
          <div>
            <div class="stat-number">
              {{
                deadlines.loading.value ? '…' : deadlines.error.value ? '—' : deadlines.total.value
              }}
            </div>
            <p class="stat-label">Upcoming deadlines</p>
            <p class="stat-detail">In the next 90 days</p>
          </div></a
        >
      </div>
      <div class="workspace-columns">
        <div class="min-w-0">
          <div class="section-heading">
            <div>
              <h2>Recently added</h2>
              <p>The latest pages in your archive.</p>
            </div>
            <RouterLink to="/documents" class="text-action"
              >View all documents<ArrowRight :size="13" aria-hidden="true"
            /></RouterLink>
          </div>
          <div v-if="recent.error.value" role="alert" class="archive-error">
            Documents could not be loaded.<Button
              variant="ghost"
              size="sm"
              @click="recent.refresh()"
              >Try again</Button
            >
          </div>
          <DocumentGrid
            v-else
            :documents="recent.documents.value"
            :loading="recent.loading.value"
            initial-view="list"
            empty-title="Your archive starts here"
            empty-message="Add your first scan or PDF. We'll turn every page into something you can find."
            empty-action="upload"
            @select-document="openDocument"
            @navigate="router.push($event)"
          />
          <div class="section-heading mt-7">
            <div>
              <h2>Find by category</h2>
              <p>Shortcuts into your document archive.</p>
            </div>
          </div>
          <div class="collection-links mt-0!">
            <RouterLink
              v-for="collection in collections"
              :key="collection.type"
              :to="{ path: '/', query: archiveQuery('', { docTypes: [collection.type] }) }"
              class="collection-link"
              ><component :is="collection.icon" :size="18" class="shrink-0" aria-hidden="true" />
              <div>
                {{ collection.label }}<small>{{ collection.hint }}</small>
              </div></RouterLink
            >
          </div>
          <RouterLink to="/chat" class="workspace-panel mt-5 flex items-center gap-4"
            ><Sparkles :size="22" class="shrink-0 text-primary" aria-hidden="true" />
            <div class="flex-1">
              <h2 class="text-sm font-medium">A question hiding in your paperwork?</h2>
              <p class="panel-caption mt-1">
                Ask your archive. Get answers with document references.
              </p>
            </div>
            <ArrowRight :size="17" class="text-primary" aria-hidden="true"
          /></RouterLink>
        </div>
        <div>
          <section id="upcoming" class="workspace-panel scroll-mt-24">
            <h2 class="panel-title">
              <CalendarDays :size="16" class="text-primary" aria-hidden="true" />On the horizon
            </h2>
            <p v-if="deadlines.loading.value" class="panel-caption" role="status">
              Checking upcoming dates…
            </p>
            <div v-else-if="deadlines.error.value" role="alert">
              <p class="panel-caption">Upcoming dates are unavailable.</p>
              <Button variant="ghost" size="sm" @click="deadlines.load(90)">Try again</Button>
            </div>
            <template v-else-if="deadlines.hasReminders.value"
              ><RouterLink
                v-for="item in deadlines.reminders.value.slice(0, 4)"
                :key="item.id"
                :to="`/documents/${encodeURIComponent(item.id)}`"
                class="reminder-item"
                ><span class="reminder-date"
                  ><small>{{ dateParts(item.dueDate).month }}</small
                  ><strong>{{ dateParts(item.dueDate).day }}</strong></span
                >
                <div class="min-w-0">
                  <h3 class="truncate">{{ item.title }}</h3>
                  <p>{{ item.company || item.docType || 'Document deadline' }}</p>
                  <p v-if="item.amountDue">{{ item.amountDue }}</p>
                </div></RouterLink
              >
              <details v-if="deadlines.reminders.value.length > 4" class="mt-3">
                <summary class="text-action cursor-pointer">
                  Show {{ deadlines.reminders.value.length - 4 }} more deadlines
                </summary>
                <RouterLink
                  v-for="item in deadlines.reminders.value.slice(4)"
                  :key="item.id"
                  :to="`/documents/${encodeURIComponent(item.id)}`"
                  class="reminder-item"
                  ><span class="reminder-date"
                    ><small>{{ dateParts(item.dueDate).month }}</small
                    ><strong>{{ dateParts(item.dueDate).day }}</strong></span
                  >
                  <div class="min-w-0">
                    <h3>{{ item.title }}</h3>
                    <p>{{ item.company || item.docType }}</p>
                  </div></RouterLink
                >
              </details></template
            >
            <div v-else class="py-3">
              <CheckCircle2 :size="24" class="mb-3 text-primary" aria-hidden="true" />
              <p class="text-sm font-medium">Nothing coming up.</p>
              <p class="panel-caption mt-1">No extracted deadlines in the next 90 days.</p>
            </div>
            <p class="reminder-footer">
              Dates extracted from your documents. Check the original before acting.
            </p>
          </section>
          <section id="saved-searches" class="workspace-panel scroll-mt-24">
            <h2 class="panel-title">
              <Bookmark :size="16" class="text-primary" aria-hidden="true" />Saved searches
            </h2>
            <div v-for="item in savedSearches" :key="item.id" class="saved-search-row">
              <button type="button" @click="navigateSearch(item.term, item.filters)">
                <Search :size="13" class="shrink-0 text-muted-foreground" aria-hidden="true" /><span
                  class="truncate"
                  >{{ item.name }}</span
                ></button
              ><button
                type="button"
                :aria-label="`Remove saved search ${item.name}`"
                @click="remove(item.id)"
              >
                <X :size="13" />
              </button>
            </div>
            <p v-if="!savedSearches.length" class="panel-caption">
              Keep your frequent searches close. Run a search, then save it here with its filters.
            </p>
            <p class="reminder-footer">Saved only in this browser on this device.</p>
            <p v-if="savedError" role="alert" class="mt-2 text-xs text-destructive">
              {{ savedError }}
            </p>
          </section>
        </div>
      </div>
    </template>
    <section v-else class="mt-7" aria-label="Search results">
      <div class="section-heading">
        <div>
          <h2>
            {{
              loading
                ? 'Searching your archive…'
                : `${formatNumber(total)} result${total === 1 ? '' : 's'}`
            }}
          </h2>
          <p>
            {{
              activeCount
                ? `${activeCount} active filter${activeCount === 1 ? '' : 's'}`
                : 'Across your full document archive'
            }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" :aria-expanded="saving" @click="startSave"
            ><Bookmark :size="14" />Save search</Button
          ><Button variant="ghost" size="sm" @click="navigateSearch('', {})"
            ><X :size="14" />Clear search</Button
          >
        </div>
      </div>
      <form v-if="saving" class="save-search-form" @submit.prevent="saveCurrent">
        <label for="save-search-name" class="text-xs">Search name</label
        ><Input
          id="save-search-name"
          v-model="saveName"
          maxlength="80"
          placeholder="e.g. This year's invoices"
          required
        /><Button type="submit" size="sm"><Plus :size="14" />Save on this device</Button
        ><Button type="button" variant="ghost" size="sm" @click="saving = false">Cancel</Button>
        <p v-if="savedError" role="alert" class="w-full text-xs text-destructive">
          {{ savedError }}
        </p>
      </form>
      <p v-if="savedMessage" role="status" class="mb-4 text-xs text-primary">{{ savedMessage }}</p>
      <div v-if="error" role="alert" class="archive-error">
        {{ error
        }}<Button
          variant="ghost"
          size="sm"
          @click="results.length ? loadMore() : search(activeTerm, filters)"
          >Try again</Button
        >
      </div>
      <div v-if="facetsError" class="mb-3 text-xs text-muted-foreground" role="status">
        Filter suggestions are unavailable.<Button variant="ghost" size="sm" @click="fetchFacets"
          >Retry filters</Button
        >
      </div>
      <div class="search-results-layout">
        <SearchFilters
          :filters="filters"
          :facets="facets"
          :loading="facetsLoading"
          :active-count="activeCount"
          @update:filters="navigateSearch(activeTerm, $event)"
          @clear="navigateSearch(activeTerm, {})"
        />
        <div class="min-w-0 flex-1">
          <DocumentGrid
            v-if="!error || results.length"
            :documents="results"
            :loading="loading"
            :loading-more="loadingMore"
            :has-more="!!scrollId && results.length < total"
            :search-term="activeTerm"
            :opensearch-url="opensearchUrl"
            empty-action="browse"
            @select-document="openDocument"
            @load-more="loadMore"
            @navigate="router.push($event)"
          />
        </div>
      </div>
    </section>
    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
