<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Home, FileText, Upload, Star, Keyboard, Command } from 'lucide-vue-next'
import { useFavorites } from '@/composables/useFavorites'

const router = useRouter()
const { count } = useFavorites()

const emit = defineEmits<{
  'open-palette': []
  'open-shortcuts': []
}>()

const navItems = [
  { name: 'Home', path: '/', icon: Home },
  { name: 'Documents', path: '/documents', icon: FileText },
  { name: 'Favorites', path: '/favorites', icon: Star },
  { name: 'Upload', path: '/upload', icon: Upload }
]

const isActive = (path: string) => router.currentRoute.value.path === path
</script>

<template>
  <aside class="fixed left-0 top-16 z-40 hidden h-[calc(100vh-4rem)] w-64 flex-col glass lg:flex" aria-label="Sidebar">
    <nav class="flex-1 overflow-auto p-4">
      <div class="space-y-1">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="flex items-center gap-3 rounded-lg px-4 py-2.5 text-sm font-medium transition-all duration-200"
          :class="[
            isActive(item.path)
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
          ]"
        >
          <component :is="item.icon" class="h-4 w-4" aria-hidden="true" />
          {{ item.name }}
          <span
            v-if="item.name === 'Favorites' && count > 0"
            class="ml-auto rounded-full bg-secondary px-1.5 py-0.5 text-xs"
          >
            {{ count }}
          </span>
          <span
            v-else-if="isActive(item.path)"
            class="ml-auto h-1.5 w-1.5 rounded-full bg-primary"
            aria-hidden="true"
          />
        </RouterLink>
      </div>

      <div class="mt-8">
        <h3 class="px-4 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Quick Actions
        </h3>
        <div class="mt-2 space-y-1">
          <button
            class="flex w-full items-center gap-3 rounded-lg px-4 py-2.5 text-sm font-medium text-muted-foreground transition-all duration-200 hover:bg-secondary hover:text-foreground"
            type="button"
            @click="emit('open-palette')"
          >
            <Command class="h-4 w-4" aria-hidden="true" />
            Command palette
            <kbd class="ml-auto hidden rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium xl:inline-block">
              ⌘K
            </kbd>
          </button>
          <button
            class="flex w-full items-center gap-3 rounded-lg px-4 py-2.5 text-sm font-medium text-muted-foreground transition-all duration-200 hover:bg-secondary hover:text-foreground"
            type="button"
            @click="emit('open-shortcuts')"
          >
            <Keyboard class="h-4 w-4" aria-hidden="true" />
            Keyboard shortcuts
            <kbd class="ml-auto hidden rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium xl:inline-block">
              ?
            </kbd>
          </button>
        </div>
      </div>
    </nav>
  </aside>
</template>
