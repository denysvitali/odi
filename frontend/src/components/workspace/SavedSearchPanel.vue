<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { Bookmark, Pencil, Search, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { SavedSearch } from '@/composables/useSavedSearches'

defineProps<{
  searches: SavedSearch[]
  error: string
  rename: (id: string, name: string) => boolean
}>()
const emit = defineEmits<{
  select: [search: SavedSearch]
  remove: [id: string]
}>()
const editingId = ref<string | null>(null)
const name = ref('')
const panel = ref<HTMLElement | null>(null)
const message = ref('')

async function startRename(search: SavedSearch) {
  editingId.value = search.id
  name.value = search.name
  message.value = ''
  await nextTick()
  const input = panel.value?.querySelector('input')
  input?.focus()
  input?.select()
}
async function closeEditor() {
  const id = editingId.value
  editingId.value = null
  await nextTick()
  const button = Array.from(
    panel.value?.querySelectorAll<HTMLButtonElement>('[data-rename-id]') || []
  ).find((item) => item.dataset.renameId === id)
  button?.focus()
}
function finishRename(rename: (id: string, name: string) => boolean) {
  if (editingId.value && rename(editingId.value, name.value)) {
    void closeEditor()
    message.value = 'Saved search renamed.'
  }
}
</script>

<template>
  <section ref="panel" id="saved-searches" class="workspace-panel scroll-mt-24">
    <h2 class="panel-title">
      <Bookmark :size="16" class="text-primary" aria-hidden="true" />Saved searches
    </h2>
    <div v-for="item in searches" :key="item.id">
      <form
        v-if="editingId === item.id"
        class="my-3 flex flex-wrap gap-2"
        @submit.prevent="finishRename(rename)"
        @keydown.esc.prevent="closeEditor"
      >
        <label :for="`saved-name-${item.id}`" class="w-full text-xs">Search name</label>
        <input
          :id="`saved-name-${item.id}`"
          v-model="name"
          class="h-9 min-w-0 flex-1 rounded-lg border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          maxlength="80"
          required
        />
        <Button type="submit" size="sm">Save name</Button>
        <Button type="button" variant="ghost" size="sm" @click="closeEditor">Cancel</Button>
      </form>
      <div v-else class="saved-search-row">
        <button type="button" @click="emit('select', item)">
          <Search :size="13" class="shrink-0 text-muted-foreground" aria-hidden="true" />
          <span class="truncate">{{ item.name }}</span>
        </button>
        <button
          type="button"
          :aria-label="`Rename saved search ${item.name}`"
          :data-rename-id="item.id"
          class="grid h-8 w-8 shrink-0 place-items-center rounded-md text-muted-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @click="startRename(item)"
        >
          <Pencil :size="13" aria-hidden="true" />
        </button>
        <button
          type="button"
          :aria-label="`Remove saved search ${item.name}`"
          class="grid h-8 w-8 shrink-0 place-items-center rounded-md hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @click="emit('remove', item.id)"
        >
          <X :size="13" aria-hidden="true" />
        </button>
      </div>
    </div>
    <p v-if="!searches.length" class="panel-caption">
      Keep your frequent searches close. Run a search, then save it here with its filters.
    </p>
    <p class="reminder-footer">Saved only in this browser on this device.</p>
    <p v-if="error" role="alert" class="mt-2 text-xs text-destructive">{{ error }}</p>
    <p v-if="message" role="status" class="mt-2 text-xs text-primary">{{ message }}</p>
  </section>
</template>
