<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { AlertCircle, CheckCircle, Loader2, RefreshCw, Search } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { api, ApiError, type ReindexStatus } from '@/api/client'

const status = ref<ReindexStatus | null>(null)
const loading = ref(false)
const starting = ref(false)
const error = ref<string | null>(null)
let pollTimer: number | undefined

const isRunning = computed(() => status.value?.state === 'running')
const completedCount = computed(() => {
  const s = status.value
  if (!s) return 0
  return s.processed + s.duplicates + s.failed
})
const progressPercent = computed(() => {
  const s = status.value
  if (!s?.total) return 0
  return Math.min(100, Math.round((completedCount.value / s.total) * 100))
})

function formatTime(value?: string) {
  if (!value) return 'Not started'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(value))
}

function clearPolling() {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

function ensurePolling() {
  if (!isRunning.value || pollTimer !== undefined) return
  pollTimer = window.setInterval(() => {
    void loadStatus()
  }, 2000)
}

async function loadStatus() {
  loading.value = true
  error.value = null
  try {
    status.value = await api.getReindexStatus()
    if (status.value.state === 'running') {
      ensurePolling()
    } else {
      clearPolling()
    }
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Unable to load reindex status'
    clearPolling()
  } finally {
    loading.value = false
  }
}

async function startReindex() {
  starting.value = true
  error.value = null
  try {
    status.value = await api.startReindex()
    ensurePolling()
  } catch (caught) {
    if (caught instanceof ApiError && caught.status === 409) {
      await loadStatus()
      return
    }
    error.value = caught instanceof Error ? caught.message : 'Unable to start reindex'
  } finally {
    starting.value = false
  }
}

onMounted(() => {
  void loadStatus()
})

onUnmounted(() => {
  clearPolling()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">Admin</h1>
        <p class="text-muted-foreground">Recover and maintain indexed document data.</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" :disabled="loading || starting" @click="loadStatus">
          <RefreshCw class="mr-2 h-4 w-4" :class="{ 'animate-spin': loading }" aria-hidden="true" />
          Refresh
        </Button>
        <Button :disabled="isRunning || starting" @click="startReindex">
          <Loader2 v-if="starting" class="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
          <Search v-else class="mr-2 h-4 w-4" aria-hidden="true" />
          Reindex from B2
        </Button>
      </div>
    </div>

    <Card v-if="error" class="border-red-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-red-600">
          <AlertCircle class="h-5 w-5" aria-hidden="true" />
          Admin action failed
        </CardTitle>
        <CardDescription>{{ error }}</CardDescription>
      </CardHeader>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <Loader2 v-if="isRunning" class="h-5 w-5 animate-spin text-primary" aria-hidden="true" />
          <CheckCircle v-else-if="status?.state === 'completed'" class="h-5 w-5 text-green-600" aria-hidden="true" />
          <AlertCircle v-else-if="status?.state === 'failed'" class="h-5 w-5 text-red-600" aria-hidden="true" />
          Reindex Status
        </CardTitle>
        <CardDescription>
          {{ status?.state || 'idle' }}
          <template v-if="status?.currentPage"> · {{ status.currentPage }}</template>
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-5">
        <div class="space-y-2">
          <div class="flex items-center justify-between text-sm">
            <span class="text-muted-foreground">Progress</span>
            <span>{{ completedCount }} / {{ status?.total || 0 }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-secondary">
            <div class="h-full bg-primary transition-all" :style="{ width: `${progressPercent}%` }" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="rounded-lg border border-border p-3">
            <div class="text-xs text-muted-foreground">Processed</div>
            <div class="text-2xl font-semibold">{{ status?.processed || 0 }}</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xs text-muted-foreground">Duplicates</div>
            <div class="text-2xl font-semibold">{{ status?.duplicates || 0 }}</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xs text-muted-foreground">Failed</div>
            <div class="text-2xl font-semibold">{{ status?.failed || 0 }}</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xs text-muted-foreground">Total</div>
            <div class="text-2xl font-semibold">{{ status?.total || 0 }}</div>
          </div>
        </div>

        <div class="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <div class="text-xs text-muted-foreground">Started</div>
            <div>{{ formatTime(status?.startedAt) }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">Finished</div>
            <div>{{ formatTime(status?.finishedAt) }}</div>
          </div>
        </div>

        <div v-if="status?.error" class="rounded-lg border border-red-500/40 p-3 text-sm text-red-600">
          {{ status.error }}
        </div>

        <div v-if="status?.recentErrors?.length" class="space-y-2">
          <h2 class="text-sm font-medium">Recent errors</h2>
          <div class="max-h-64 space-y-2 overflow-auto">
            <div v-for="item in status.recentErrors" :key="`${item.page}-${item.error}`" class="rounded-lg border border-border p-3 text-sm">
              <div class="font-medium">{{ item.page }}</div>
              <div class="break-words text-muted-foreground">{{ item.error }}</div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
