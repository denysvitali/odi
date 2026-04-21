<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterView, useRouter } from 'vue-router'
import AppHeader from '@/components/layout/AppHeader.vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import ErrorBoundary from '@/components/error/ErrorBoundary.vue'
import NetworkBanner from '@/components/layout/NetworkBanner.vue'
import ShortcutsDialog from '@/components/layout/ShortcutsDialog.vue'
import CommandPalette from '@/components/layout/CommandPalette.vue'
import GlobalDropOverlay from '@/components/layout/GlobalDropOverlay.vue'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const showShortcuts = ref(false)
const showPalette = ref(false)

// Activate theme on mount
useTheme()

let goBuffer: string | null = null
let goTimer: ReturnType<typeof setTimeout> | null = null

useKeyboardShortcuts([
  {
    key: 'k',
    meta: true,
    description: 'Open command palette',
    handler: () => (showPalette.value = true),
    allowInInput: true
  },
  {
    key: '?',
    shift: true,
    description: 'Show shortcuts',
    handler: () => (showShortcuts.value = true)
  },
  {
    key: '/',
    description: 'Focus search',
    handler: () => {
      const el = document.querySelector<HTMLInputElement>('input[data-global-search]')
      if (el) el.focus()
      else router.push('/')
    }
  },
  {
    key: 'Escape',
    description: 'Close dialogs',
    handler: () => {
      if (showPalette.value) showPalette.value = false
      else if (showShortcuts.value) showShortcuts.value = false
    },
    allowInInput: true
  },
  {
    key: 'g',
    description: 'Go to…',
    handler: () => {
      goBuffer = 'g'
      if (goTimer) clearTimeout(goTimer)
      goTimer = setTimeout(() => {
        goBuffer = null
      }, 1000)
    }
  },
  {
    key: 'h',
    description: 'Go home',
    handler: () => {
      if (goBuffer === 'g') router.push('/')
      goBuffer = null
    }
  },
  {
    key: 'd',
    description: 'Go to documents',
    handler: () => {
      if (goBuffer === 'g') router.push('/documents')
      goBuffer = null
    }
  },
  {
    key: 'u',
    description: 'Go to upload',
    handler: () => {
      if (goBuffer === 'g') router.push('/upload')
      goBuffer = null
    }
  },
  {
    key: 'f',
    description: 'Go to favorites',
    handler: () => {
      if (goBuffer === 'g') router.push('/favorites')
      goBuffer = null
    }
  }
])

watch(
  () => router.currentRoute.value.path,
  () => {
    showPalette.value = false
    showShortcuts.value = false
  }
)
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <a
      href="#main"
      class="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-[200] focus:rounded-md focus:bg-background focus:px-3 focus:py-2 focus:text-sm focus:shadow"
    >
      Skip to content
    </a>
    <AppHeader @open-palette="showPalette = true" @open-shortcuts="showShortcuts = true" />
    <NetworkBanner />

    <div class="flex">
      <AppSidebar @open-palette="showPalette = true" @open-shortcuts="showShortcuts = true" />

      <main id="main" class="flex-1 lg:ml-64">
        <div class="p-4 lg:p-8">
          <ErrorBoundary>
            <RouterView v-slot="{ Component }">
              <Transition
                mode="out-in"
                enter-active-class="transition-all duration-300 ease-out"
                leave-active-class="transition-all duration-200 ease-in"
                enter-from-class="opacity-0 translate-y-2"
                leave-to-class="opacity-0 -translate-y-2"
              >
                <component :is="Component" />
              </Transition>
            </RouterView>
          </ErrorBoundary>
        </div>
      </main>
    </div>

    <CommandPalette
      :open="showPalette"
      @update:open="showPalette = $event"
      @show-shortcuts="() => { showPalette = false; showShortcuts = true }"
    />
    <ShortcutsDialog :open="showShortcuts" @update:open="showShortcuts = $event" />
    <GlobalDropOverlay />
  </div>
</template>
