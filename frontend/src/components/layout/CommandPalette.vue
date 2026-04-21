<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Search, FileText, Home, Upload, Sun, Moon, Keyboard, Trash2 } from 'lucide-vue-next'
import { useTheme } from '@/composables/useTheme'
import { useDocumentStore } from '@/stores/documents'

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
}>()

const router = useRouter()
const { toggleTheme, isDark } = useTheme()
const store = useDocumentStore()

const query = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const selected = ref(0)

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

watch(
  () => props.open,
  (open) => {
    if (open) {
      query.value = ''
      selected.value = 0
      nextTick(() => inputRef.value?.focus())
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

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    selected.value = Math.min(selected.value + 1, filtered.value.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    selected.value = Math.max(selected.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const a = filtered.value[selected.value]
    if (a) run(a)
    else if (query.value.trim()) {
      router.push({ path: '/', query: { q: query.value.trim() } })
      close()
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
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
            <li v-if="filtered.length === 0" class="px-3 py-6 text-center text-sm text-muted-foreground">
              No matches. Press Enter to search for "{{ query }}".
            </li>
          </ul>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
