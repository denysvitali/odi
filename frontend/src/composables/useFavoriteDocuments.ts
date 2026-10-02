import { computed, onScopeDispose, reactive, watch, type Ref } from 'vue'
import { api } from '@/api/client'
import type { Document, DocumentDetails } from '@/types/documents'

type Entry = {
  status: 'pending' | 'loading' | 'loaded' | 'error'
  document?: Document
  skipCache?: boolean
}

function toDocument(id: string, details: DocumentDetails): Document {
  return {
    _id: id,
    _source: {
      title: details.title,
      text: details.text || '',
      date: details.primaryDate,
      indexedAt: details.indexedAt,
      company: details.company,
      docType: details.docType,
      tags: details.tags,
      summary: details.summary,
      keyFacts: details.keyFacts
    }
  }
}

/** Resolve saved IDs directly, including records outside the archive's first page. */
export function useFavoriteDocuments(ids: Ref<string[]>) {
  const entries = reactive(new Map<string, Entry>())
  let active = 0
  let disposed = false
  const concurrency = 4

  const pump = () => {
    if (disposed) return
    for (const id of ids.value) {
      if (active >= concurrency) break
      const entry = entries.get(id)
      if (!entry || entry.status !== 'pending') continue
      entry.status = 'loading'
      active++
      void api
        .getDocumentDetails(id, { skipCache: !!entry.skipCache })
        .then((details) => {
          if (disposed || entries.get(id) !== entry) return
          entry.document = toDocument(id, details)
          entry.status = 'loaded'
        })
        .catch(() => {
          if (!disposed && entries.get(id) === entry) entry.status = 'error'
        })
        .finally(() => {
          active--
          pump()
        })
    }
  }

  watch(
    ids,
    (current) => {
      const retained = new Set(current)
      for (const id of entries.keys()) if (!retained.has(id)) entries.delete(id)
      for (const id of current) if (!entries.has(id)) entries.set(id, { status: 'pending' })
      pump()
    },
    { immediate: true }
  )

  onScopeDispose(() => {
    disposed = true
    entries.clear()
  })

  const documents = computed(() =>
    ids.value.flatMap((id) => {
      const doc = entries.get(id)?.document
      return doc ? [doc] : []
    })
  )
  const failedCount = computed(
    () => ids.value.filter((id) => entries.get(id)?.status === 'error').length
  )
  const pendingCount = computed(
    () =>
      ids.value.filter((id) => ['pending', 'loading'].includes(entries.get(id)?.status || ''))
        .length
  )
  const retry = () => {
    for (const entry of entries.values()) {
      if (entry.status === 'error') {
        entry.status = 'pending'
        entry.skipCache = true
      }
    }
    pump()
  }

  return {
    documents,
    failedCount,
    pendingCount,
    loading: computed(() => pendingCount.value > 0),
    retry
  }
}
