import { ref, computed } from 'vue'
import { STORAGE_KEYS } from '@/lib/constants'

type TagMap = Record<string, string[]>

const tagsByDoc = ref<TagMap>({})
let loaded = false

function save() {
  try {
    localStorage.setItem(STORAGE_KEYS.TAGS, JSON.stringify(tagsByDoc.value))
  } catch {}
}

function load() {
  if (loaded) return
  loaded = true
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.TAGS)
    if (raw) tagsByDoc.value = JSON.parse(raw)
  } catch {}
}

export function useTags() {
  load()

  const allTags = computed(() => {
    const set = new Set<string>()
    for (const arr of Object.values(tagsByDoc.value) as string[][]) {
      for (const t of arr) set.add(t)
    }
    return [...set].sort()
  })

  const getTags = (docId: string): string[] => tagsByDoc.value[docId] || []

  const addTag = (docId: string, tag: string) => {
    const clean = tag.trim()
    if (!clean) return
    const current = tagsByDoc.value[docId] || []
    if (current.includes(clean)) return
    tagsByDoc.value = { ...tagsByDoc.value, [docId]: [...current, clean] }
    save()
  }

  const removeTag = (docId: string, tag: string) => {
    const current = tagsByDoc.value[docId] || []
    tagsByDoc.value = { ...tagsByDoc.value, [docId]: current.filter((t: string) => t !== tag) }
    save()
  }

  return { allTags, getTags, addTag, removeTag }
}
