import { ref, computed } from 'vue'
import { STORAGE_KEYS } from '@/lib/constants'

const favorites = ref<Set<string>>(new Set())
let loaded = false

function save() {
  try {
    localStorage.setItem(STORAGE_KEYS.FAVORITES, JSON.stringify([...favorites.value]))
  } catch {}
}

function load() {
  if (loaded) return
  loaded = true
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.FAVORITES)
    if (raw) favorites.value = new Set(JSON.parse(raw))
  } catch {}
}

export function useFavorites() {
  load()

  const list = computed(() => [...favorites.value])
  const count = computed(() => favorites.value.size)

  const isFavorite = (id: string) => favorites.value.has(id)

  const toggle = (id: string) => {
    if (favorites.value.has(id)) favorites.value.delete(id)
    else favorites.value.add(id)
    favorites.value = new Set(favorites.value)
    save()
  }

  const clear = () => {
    favorites.value = new Set()
    save()
  }

  return { list, count, isFavorite, toggle, clear }
}
