<script setup lang="ts">
import { ref, computed } from 'vue'
import { AlertCircle, RefreshCw, Copy, Check } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

interface Props {
  title?: string
  message: string
  retryable?: boolean
  lastRetryAt?: Date | null
  onRetry?: () => void
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Something went wrong',
  retryable: true,
  lastRetryAt: null
})

const copied = ref(false)

const formattedLastRetry = computed(() => {
  if (!props.lastRetryAt) return null
  const now = new Date()
  const diffMs = now.getTime() - props.lastRetryAt.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)

  if (diffSec < 60) return 'just now'
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  return props.lastRetryAt.toLocaleDateString()
})

const copyErrorDetails = async () => {
  const details = {
    message: props.message,
    timestamp: new Date().toISOString(),
    url: window.location.href
  }
  try {
    await navigator.clipboard.writeText(JSON.stringify(details, null, 2))
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // Fallback for older browsers
    const textarea = document.createElement('textarea')
    textarea.value = JSON.stringify(details, null, 2)
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  }
}

const friendlyMessage = computed(() => {
  if (props.message.includes('Failed to fetch') || props.message.includes('Network')) {
    return 'Unable to connect to the server. Please check your internet connection.'
  }
  if (props.message.includes('401') || props.message.includes('Unauthorized')) {
    return 'Your session may have expired. Please try refreshing the page.'
  }
  if (props.message.includes('403') || props.message.includes('Forbidden')) {
    return 'You do not have permission to view this content.'
  }
  if (props.message.includes('404') || props.message.includes('Not Found')) {
    return 'The requested resource could not be found.'
  }
  if (props.message.includes('500') || props.message.includes('Internal Server Error')) {
    return 'The server encountered an unexpected error. Please try again later.'
  }
  return props.message
})
</script>

<template>
  <div class="rounded-lg border border-destructive/30 bg-destructive/5 p-6 text-center">
    <AlertCircle class="mx-auto h-10 w-10 text-destructive/70 mb-4" />

    <h3 class="text-lg font-semibold text-foreground mb-1">
      {{ title }}
    </h3>

    <p class="text-sm text-muted-foreground mb-4 max-w-sm mx-auto">
      {{ friendlyMessage }}
    </p>

    <div v-if="retryable && onRetry" class="flex flex-col items-center gap-3">
      <Button
        variant="outline"
        size="sm"
        @click="onRetry"
      >
        <RefreshCw class="mr-2 h-4 w-4" />
        Try Again
      </Button>

      <p v-if="lastRetryAt" class="text-xs text-muted-foreground">
        Last attempt: {{ formattedLastRetry }}
      </p>
    </div>

    <div class="mt-4 pt-4 border-t border-border/50">
      <Button
        variant="ghost"
        size="sm"
        @click="copyErrorDetails"
      >
        <component :is="copied ? Check : Copy" class="mr-2 h-4 w-4" />
        {{ copied ? 'Copied!' : 'Copy error details' }}
      </Button>
    </div>
  </div>
</template>
