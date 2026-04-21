<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Upload, CheckCircle, XCircle, Loader2, X, Camera, RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useUpload } from '@/composables/useUpload'
import { formatBytes } from '@/lib/format'

const router = useRouter()
const { uploading, progress, result, error, attempt, upload, abort, reset } = useUpload()

const selectedFiles = ref<File[]>([])
const previews = ref<Map<File, string>>(new Map())
const isDragging = ref(false)

const totalSize = computed(() => selectedFiles.value.reduce((a, f) => a + f.size, 0))

function previewFor(file: File): string {
  let url = previews.value.get(file)
  if (!url) {
    url = URL.createObjectURL(file)
    previews.value.set(file, url)
  }
  return url
}

function revokePreview(file: File) {
  const url = previews.value.get(file)
  if (url) {
    URL.revokeObjectURL(url)
    previews.value.delete(file)
  }
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}
function onDragLeave() {
  isDragging.value = false
}
function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  if (!e.dataTransfer?.files) return
  addFiles(Array.from(e.dataTransfer.files))
}
function onFileInput(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  addFiles(Array.from(input.files))
  input.value = ''
}
function addFiles(files: File[]) {
  const imageFiles = files.filter((f) => f.type.startsWith('image/'))
  selectedFiles.value.push(...imageFiles)
}
function removeFile(index: number) {
  const [removed] = selectedFiles.value.splice(index, 1)
  if (removed) revokePreview(removed)
}
function clearAll() {
  selectedFiles.value.forEach(revokePreview)
  selectedFiles.value = []
}
async function handleUpload() {
  if (selectedFiles.value.length === 0) return
  try {
    await upload(selectedFiles.value)
  } catch {
    // error already surfaced via composable
  }
}
async function retry() {
  if (selectedFiles.value.length === 0) return
  await handleUpload()
}
function handleReset() {
  reset()
  clearAll()
}
function goToDocuments() {
  router.push('/documents')
}

onMounted(() => {
  const pending = (window as unknown as { __pendingUpload?: File[] }).__pendingUpload
  if (pending?.length) {
    addFiles(pending)
    ;(window as unknown as { __pendingUpload?: File[] }).__pendingUpload = undefined
  }
})

onUnmounted(() => {
  selectedFiles.value.forEach(revokePreview)
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight">Upload Documents</h1>
      <p class="text-muted-foreground">Upload scanned images for OCR processing and indexing.</p>
    </div>

    <Card v-if="result && result.failed === 0" class="border-green-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-green-600">
          <CheckCircle class="h-5 w-5" aria-hidden="true" />
          Upload Complete
        </CardTitle>
        <CardDescription>
          Processed {{ result.processed }} page(s)
          <template v-if="result.duplicates">, skipped {{ result.duplicates }} duplicate(s)</template>.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="text-sm text-muted-foreground">
          Scan ID: <code class="rounded bg-muted px-1.5 py-0.5">{{ result.scanID }}</code>
        </div>
        <div class="flex gap-2">
          <Button @click="goToDocuments">View Documents</Button>
          <Button variant="outline" @click="handleReset">Upload More</Button>
        </div>
      </CardContent>
    </Card>

    <Card v-if="result && result.failed > 0" class="border-yellow-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-yellow-600">
          <XCircle class="h-5 w-5" aria-hidden="true" />
          Upload Completed with Errors
        </CardTitle>
        <CardDescription>
          {{ result.processed }} succeeded, {{ result.duplicates }} duplicate(s), {{ result.failed }} failed.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="text-sm text-muted-foreground">
          Scan ID: <code class="rounded bg-muted px-1.5 py-0.5">{{ result.scanID }}</code>
        </div>
        <div class="space-y-1">
          <div v-for="page in result.pages" :key="page.sequenceID" class="flex items-center gap-2 text-sm">
            <CheckCircle
              v-if="page.status === 'indexed' || page.status === 'duplicate'"
              class="h-4 w-4 text-green-500"
              aria-hidden="true"
            />
            <XCircle v-else class="h-4 w-4 text-red-500" aria-hidden="true" />
            <span>Page {{ page.sequenceID }}: {{ page.status }}</span>
            <span v-if="page.duplicateOf" class="text-muted-foreground">(duplicate of {{ page.duplicateOf }})</span>
            <span v-if="page.error" class="text-muted-foreground">({{ page.error }})</span>
          </div>
        </div>
        <div class="flex gap-2">
          <Button @click="goToDocuments">View Documents</Button>
          <Button variant="outline" @click="handleReset">Upload More</Button>
        </div>
      </CardContent>
    </Card>

    <template v-if="!result">
      <div
        class="rounded-xl border-2 border-dashed transition-all"
        :class="isDragging ? 'border-primary bg-primary/5 scale-[1.01]' : 'border-border'"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <CardContent class="flex flex-col items-center justify-center py-12">
          <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <Upload class="h-7 w-7" aria-hidden="true" />
          </div>
          <p class="mb-1 text-sm font-medium">Drag and drop images here</p>
          <p class="mb-4 text-xs text-muted-foreground">or click to browse files</p>
          <div class="flex gap-2">
            <label>
              <input
                type="file"
                accept="image/*"
                multiple
                class="hidden"
                aria-label="Select files"
                @change="onFileInput"
              />
              <Button variant="outline" size="sm" as="span" class="cursor-pointer">
                Browse Files
              </Button>
            </label>
            <label>
              <input
                type="file"
                accept="image/*"
                capture="environment"
                class="hidden"
                aria-label="Capture from camera"
                @change="onFileInput"
              />
              <Button variant="ghost" size="sm" as="span" class="cursor-pointer">
                <Camera class="mr-2 h-4 w-4" aria-hidden="true" />
                Use Camera
              </Button>
            </label>
          </div>
        </CardContent>
      </div>

      <Card v-if="selectedFiles.length > 0">
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base">
              {{ selectedFiles.length }} file(s) selected
              <span class="font-normal text-muted-foreground">({{ formatBytes(totalSize) }})</span>
            </CardTitle>
            <Button variant="ghost" size="sm" :disabled="uploading" @click="clearAll">Clear All</Button>
          </div>
        </CardHeader>
        <CardContent>
          <div class="grid max-h-96 grid-cols-3 gap-2 overflow-auto sm:grid-cols-4 md:grid-cols-5">
            <div
              v-for="(file, i) in selectedFiles"
              :key="i"
              class="group relative aspect-square overflow-hidden rounded-lg border bg-muted"
            >
              <img
                :src="previewFor(file)"
                :alt="file.name"
                class="h-full w-full object-cover"
                loading="lazy"
                decoding="async"
              />
              <div class="absolute inset-x-0 bottom-0 truncate bg-gradient-to-t from-black/70 to-transparent px-2 pb-1 pt-6 text-[10px] text-white">
                {{ file.name }}
              </div>
              <button
                type="button"
                class="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-background/80 text-foreground opacity-0 backdrop-blur transition-opacity hover:bg-background group-hover:opacity-100"
                :aria-label="`Remove ${file.name}`"
                :disabled="uploading"
                @click="removeFile(i)"
              >
                <X class="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div v-if="uploading" class="mt-4 space-y-2">
            <div class="h-2 overflow-hidden rounded-full bg-secondary">
              <div
                class="h-full rounded-full bg-gradient-to-r from-primary to-apple-purple transition-all duration-300"
                :style="{ width: `${progress}%` }"
              />
            </div>
            <div class="flex items-center justify-between text-xs text-muted-foreground">
              <span class="inline-flex items-center gap-1">
                <Loader2 class="h-3 w-3 animate-spin" aria-hidden="true" />
                Uploading… {{ progress }}%
                <span v-if="attempt > 1">· attempt {{ attempt }}</span>
              </span>
              <Button variant="ghost" size="sm" class="h-6 text-xs" @click="abort">Cancel</Button>
            </div>
          </div>

          <Button
            v-else
            class="mt-4 w-full"
            :disabled="selectedFiles.length === 0"
            @click="handleUpload"
          >
            <Upload class="mr-2 h-4 w-4" aria-hidden="true" />
            Upload {{ selectedFiles.length }} file(s)
          </Button>
        </CardContent>
      </Card>

      <Card v-if="error" class="border-red-500/50">
        <CardContent class="py-4">
          <div class="flex flex-wrap items-center gap-2 text-red-600">
            <XCircle class="h-5 w-5" aria-hidden="true" />
            <p class="flex-1 text-sm">{{ error }}</p>
            <Button variant="outline" size="sm" @click="retry">
              <RefreshCw class="mr-2 h-3.5 w-3.5" aria-hidden="true" />
              Retry
            </Button>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
