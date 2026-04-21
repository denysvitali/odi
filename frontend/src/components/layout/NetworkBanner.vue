<script setup lang="ts">
import { WifiOff } from 'lucide-vue-next'
import { useNetworkStatus } from '@/composables/useNetworkStatus'

const { online } = useNetworkStatus()
</script>

<template>
  <Transition
    enter-active-class="transition-all duration-300"
    enter-from-class="-translate-y-full opacity-0"
    leave-active-class="transition-all duration-200"
    leave-to-class="-translate-y-full opacity-0"
  >
    <div
      v-if="!online"
      role="status"
      aria-live="polite"
      class="fixed left-1/2 top-20 z-[100] -translate-x-1/2 rounded-full border border-yellow-500/40 bg-yellow-500/10 px-4 py-2 text-sm font-medium text-yellow-700 shadow-lg backdrop-blur-md dark:text-yellow-300"
    >
      <div class="flex items-center gap-2">
        <WifiOff class="h-4 w-4" aria-hidden="true" />
        You're offline. Changes may not save.
      </div>
    </div>
  </Transition>
</template>
