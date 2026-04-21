<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const error = ref<Error | null>(null)

onErrorCaptured((err) => {
  error.value = err as Error
  return false
})

const reset = () => {
  error.value = null
}

const reload = () => {
  window.location.reload()
}
</script>

<template>
  <div v-if="error" class="flex min-h-[60vh] items-center justify-center p-6">
    <div class="max-w-md text-center">
      <div class="mb-4 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
        <svg class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>
      <h2 class="mb-2 text-xl font-semibold tracking-tight">Something went wrong</h2>
      <p class="mb-4 text-sm text-muted-foreground">
        The app hit an unexpected error. You can try again or reload the page.
      </p>
      <pre class="mb-4 max-h-40 overflow-auto rounded-lg bg-muted p-3 text-left text-xs text-muted-foreground">{{ error.message }}</pre>
      <div class="flex justify-center gap-2">
        <Button variant="outline" size="sm" @click="reset">
          <RefreshCw class="mr-2 h-4 w-4" aria-hidden="true" />
          Try Again
        </Button>
        <Button size="sm" @click="reload">Reload Page</Button>
      </div>
    </div>
  </div>
  <slot v-else />
</template>
