<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Menu, X, FileText, Home, Upload, Star, Sun, Moon, Command, Keyboard } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/composables/useTheme'

const emit = defineEmits<{
  'open-palette': []
  'open-shortcuts': []
}>()

const router = useRouter()
const { toggleTheme, isDark } = useTheme()
const mobileMenuOpen = ref(false)

const navItems = [
  { name: 'Home', path: '/', icon: Home },
  { name: 'Documents', path: '/documents', icon: FileText },
  { name: 'Favorites', path: '/favorites', icon: Star },
  { name: 'Upload', path: '/upload', icon: Upload }
]

const isActive = (path: string) => router.currentRoute.value.path === path
</script>

<template>
  <header class="sticky top-0 z-50 w-full glass">
    <div class="flex h-16 items-center justify-between px-4 lg:px-8">
      <div class="flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon"
          class="lg:hidden"
          aria-label="Toggle menu"
          :aria-expanded="mobileMenuOpen"
          @click="mobileMenuOpen = !mobileMenuOpen"
        >
          <Menu v-if="!mobileMenuOpen" class="h-5 w-5" aria-hidden="true" />
          <X v-else class="h-5 w-5" aria-hidden="true" />
        </Button>
        <RouterLink to="/" class="flex items-center gap-2">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-apple-purple text-primary-foreground shadow-sm">
            <FileText class="h-4 w-4" aria-hidden="true" />
          </div>
          <span class="text-lg font-semibold tracking-tight">ODI</span>
        </RouterLink>
      </div>

      <nav class="hidden lg:flex items-center gap-1" aria-label="Primary">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="relative px-4 py-2 text-sm font-medium transition-colors rounded-lg"
          :class="[
            isActive(item.path)
              ? 'text-foreground bg-secondary'
              : 'text-muted-foreground hover:text-foreground hover:bg-secondary/50'
          ]"
        >
          <span class="flex items-center gap-2">
            <component :is="item.icon" class="h-4 w-4" aria-hidden="true" />
            {{ item.name }}
          </span>
        </RouterLink>
      </nav>

      <div class="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Open command palette"
          class="text-muted-foreground"
          @click="emit('open-palette')"
        >
          <Command class="h-5 w-5" aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Show keyboard shortcuts"
          class="hidden text-muted-foreground sm:inline-flex"
          @click="emit('open-shortcuts')"
        >
          <Keyboard class="h-5 w-5" aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
          class="text-muted-foreground"
          @click="toggleTheme"
        >
          <Sun v-if="isDark" class="h-5 w-5" aria-hidden="true" />
          <Moon v-else class="h-5 w-5" aria-hidden="true" />
        </Button>
      </div>
    </div>

    <div v-show="mobileMenuOpen" class="lg:hidden border-t border-border">
      <nav class="flex flex-col p-4 space-y-1" aria-label="Mobile">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="flex items-center gap-3 px-4 py-3 text-sm font-medium rounded-lg transition-colors"
          :class="[
            isActive(item.path)
              ? 'bg-secondary text-foreground'
              : 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'
          ]"
          @click="mobileMenuOpen = false"
        >
          <component :is="item.icon" class="h-4 w-4" aria-hidden="true" />
          {{ item.name }}
        </RouterLink>
      </nav>
    </div>
  </header>
</template>
