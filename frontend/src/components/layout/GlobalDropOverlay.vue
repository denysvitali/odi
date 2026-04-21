<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Upload } from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const dragging = ref(false)
const counter = ref(0)

const hasFiles = (e: DragEvent) =>
  !!e.dataTransfer && (e.dataTransfer.types?.includes('Files') || false)

const onDragEnter = (e: DragEvent) => {
  if (!hasFiles(e)) return
  counter.value++
  dragging.value = true
}
const onDragOver = (e: DragEvent) => {
  if (!hasFiles(e)) return
  e.preventDefault()
}
const onDragLeave = () => {
  counter.value = Math.max(0, counter.value - 1)
  if (counter.value === 0) dragging.value = false
}
const onDrop = (e: DragEvent) => {
  if (!hasFiles(e)) return
  e.preventDefault()
  dragging.value = false
  counter.value = 0
  // Don't intercept drops on the upload view itself — it has its own handling.
  if (route.path === '/upload') return
  const files = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith('image/'))
  if (files.length === 0) return
  // Stash files on window so UploadView can pick them up after navigation.
  ;(window as unknown as { __pendingUpload?: File[] }).__pendingUpload = files
  router.push('/upload')
}

onMounted(() => {
  window.addEventListener('dragenter', onDragEnter)
  window.addEventListener('dragover', onDragOver)
  window.addEventListener('dragleave', onDragLeave)
  window.addEventListener('drop', onDrop)
})
onUnmounted(() => {
  window.removeEventListener('dragenter', onDragEnter)
  window.removeEventListener('dragover', onDragOver)
  window.removeEventListener('dragleave', onDragLeave)
  window.removeEventListener('drop', onDrop)
})
</script>

<template>
  <Transition
    enter-active-class="transition-opacity duration-200"
    leave-active-class="transition-opacity duration-150"
    enter-from-class="opacity-0"
    leave-to-class="opacity-0"
  >
    <div
      v-if="dragging"
      class="pointer-events-none fixed inset-0 z-[80] flex items-center justify-center bg-primary/10 backdrop-blur-sm"
      aria-hidden="true"
    >
      <div class="rounded-3xl border-2 border-dashed border-primary/60 bg-background/80 px-10 py-8 text-center shadow-2xl">
        <Upload class="mx-auto mb-3 h-10 w-10 text-primary" />
        <p class="text-lg font-semibold">Drop to upload</p>
        <p class="text-sm text-muted-foreground">Images will be sent for OCR indexing</p>
      </div>
    </div>
  </Transition>
</template>
