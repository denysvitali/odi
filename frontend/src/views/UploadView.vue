<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Upload, FileImage, CheckCircle, XCircle, Loader2, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useUpload } from '@/composables/useUpload'

const router = useRouter()
const { uploading, progress, result, error, upload, reset } = useUpload()

const selectedFiles = ref<File[]>([])
const isDragging = ref(false)

const totalSize = computed(() => {
  return selectedFiles.value.reduce((acc, f) => acc + f.size, 0)
})

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
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
  const imageFiles = files.filter(f => f.type.startsWith('image/'))
  selectedFiles.value.push(...imageFiles)
}

function removeFile(index: number) {
  selectedFiles.value.splice(index, 1)
}

async function handleUpload() {
  if (selectedFiles.value.length === 0) return
  await upload(selectedFiles.value)
}

function handleReset() {
  reset()
  selectedFiles.value = []
}

function goToDocuments() {
  router.push('/documents')
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight">Upload Documents</h1>
      <p class="text-muted-foreground">Upload scanned JPG images for OCR processing and indexing.</p>
    </div>

    <!-- Success state -->
    <Card v-if="result && result.failed === 0" class="border-green-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-green-600">
          <CheckCircle class="h-5 w-5" />
          Upload Complete
        </CardTitle>
        <CardDescription>
          Successfully processed {{ result.processed }} page(s).
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

    <!-- Partial success / failure state -->
    <Card v-if="result && result.failed > 0" class="border-yellow-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-yellow-600">
          <XCircle class="h-5 w-5" />
          Upload Completed with Errors
        </CardTitle>
        <CardDescription>
          {{ result.processed }} succeeded, {{ result.failed }} failed.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="text-sm text-muted-foreground">
          Scan ID: <code class="rounded bg-muted px-1.5 py-0.5">{{ result.scanID }}</code>
        </div>
        <div class="space-y-1">
          <div v-for="page in result.pages" :key="page.sequenceID" class="flex items-center gap-2 text-sm">
            <CheckCircle v-if="page.status === 'indexed'" class="h-4 w-4 text-green-500" />
            <XCircle v-else class="h-4 w-4 text-red-500" />
            <span>Page {{ page.sequenceID }}: {{ page.status }}</span>
            <span v-if="page.error" class="text-muted-foreground">({{ page.error }})</span>
          </div>
        </div>
        <div class="flex gap-2">
          <Button @click="goToDocuments">View Documents</Button>
          <Button variant="outline" @click="handleReset">Upload More</Button>
        </div>
      </CardContent>
    </Card>

    <!-- Upload form (shown when no result yet) -->
    <template v-if="!result">
      <!-- Drop zone -->
      <div
        class="rounded-xl border transition-colors"
        :class="isDragging ? 'border-primary bg-primary/5' : 'border-dashed'"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <CardContent class="flex flex-col items-center justify-center py-12">
          <Upload class="h-10 w-10 text-muted-foreground mb-4" />
          <p class="text-sm font-medium mb-1">
            Drag and drop images here
          </p>
          <p class="text-xs text-muted-foreground mb-4">
            or click to browse files
          </p>
          <label>
            <input
              type="file"
              accept="image/*"
              multiple
              class="hidden"
              @change="onFileInput"
            />
            <Button variant="outline" size="sm" as="span" class="cursor-pointer">
              Browse Files
            </Button>
          </label>
          <label class="mt-2">
            <input
              type="file"
              accept="image/*"
              capture="environment"
              class="hidden"
              @change="onFileInput"
            />
            <Button variant="ghost" size="sm" as="span" class="cursor-pointer text-xs">
              Use Camera
            </Button>
          </label>
        </CardContent>
      </div>

      <!-- Selected files -->
      <Card v-if="selectedFiles.length > 0">
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base">
              {{ selectedFiles.length }} file(s) selected
              <span class="text-muted-foreground font-normal">({{ formatSize(totalSize) }})</span>
            </CardTitle>
            <Button variant="ghost" size="sm" @click="selectedFiles = []">Clear All</Button>
          </div>
        </CardHeader>
        <CardContent>
          <div class="space-y-2 max-h-64 overflow-auto">
            <div
              v-for="(file, i) in selectedFiles"
              :key="i"
              class="flex items-center gap-3 rounded-lg border p-2"
            >
              <FileImage class="h-5 w-5 text-muted-foreground shrink-0" />
              <div class="flex-1 min-w-0">
                <p class="text-sm truncate">{{ file.name }}</p>
                <p class="text-xs text-muted-foreground">{{ formatSize(file.size) }}</p>
              </div>
              <Button variant="ghost" size="icon" class="h-7 w-7 shrink-0" @click="removeFile(i)">
                <X class="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>

          <!-- Progress bar -->
          <div v-if="uploading" class="mt-4 space-y-2">
            <div class="h-2 rounded-full bg-secondary overflow-hidden">
              <div
                class="h-full bg-primary transition-all duration-300 rounded-full"
                :style="{ width: `${progress}%` }"
              />
            </div>
            <p class="text-xs text-muted-foreground text-center">
              <Loader2 class="inline h-3 w-3 animate-spin mr-1" />
              Uploading... {{ progress }}%
            </p>
          </div>

          <Button
            v-else
            class="mt-4 w-full"
            :disabled="selectedFiles.length === 0"
            @click="handleUpload"
          >
            <Upload class="h-4 w-4 mr-2" />
            Upload {{ selectedFiles.length }} file(s)
          </Button>
        </CardContent>
      </Card>

      <!-- Error message -->
      <Card v-if="error" class="border-red-500/50">
        <CardContent class="py-4">
          <div class="flex items-center gap-2 text-red-600">
            <XCircle class="h-5 w-5" />
            <p class="text-sm">{{ error }}</p>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
