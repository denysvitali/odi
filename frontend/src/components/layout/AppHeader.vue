<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Menu, Search, Sun, Moon, Plus, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import AppSidebar from './AppSidebar.vue'
import { useTheme } from '@/composables/useTheme'

const emit = defineEmits<{ 'open-palette': []; 'open-shortcuts': [] }>()
const route = useRoute()
const { toggleTheme, isDark } = useTheme()
const menuOpen = ref(false)
watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  }
)
function openTool(tool: 'open-palette' | 'open-shortcuts') {
  menuOpen.value = false
  if (tool === 'open-palette') emit('open-palette')
  else emit('open-shortcuts')
}
</script>

<template>
  <header class="archive-header">
    <div class="flex min-w-0 items-center gap-3">
      <Button
        variant="ghost"
        size="icon"
        class="lg:hidden"
        aria-label="Open navigation"
        :aria-expanded="menuOpen"
        @click="menuOpen = true"
        ><Menu :size="20"
      /></Button>
      <span class="hidden text-sm text-muted-foreground sm:inline">Personal archive</span>
      <ChevronRight :size="14" class="hidden text-muted-foreground sm:block" aria-hidden="true" />
      <span class="truncate text-sm font-medium">{{
        route.path === '/' ? 'Overview' : route.meta.title
      }}</span>
    </div>
    <div class="flex items-center gap-2">
      <button
        type="button"
        class="header-search"
        aria-label="Open command palette"
        @click="emit('open-palette')"
      >
        <Search :size="16" aria-hidden="true" /><span class="hidden sm:inline"
          >Jump to anything…</span
        ><kbd class="hidden md:inline">Ctrl / ⌘ K</kbd>
      </button>
      <Button
        variant="ghost"
        size="icon"
        :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
        @click="toggleTheme"
        ><Sun v-if="isDark" :size="18" /><Moon v-else :size="18"
      /></Button>
      <RouterLink to="/upload" class="header-add"
        ><Plus :size="16" aria-hidden="true" /><span class="hidden sm:inline">Add documents</span
        ><span class="sr-only sm:hidden">Add documents</span></RouterLink
      >
    </div>
  </header>
  <Sheet v-model:open="menuOpen">
    <SheetContent side="left" class="mobile-navigation p-0">
      <SheetTitle class="sr-only">Archive navigation</SheetTitle
      ><SheetDescription class="sr-only">Navigate your document workspace.</SheetDescription>
      <AppSidebar
        @navigate="menuOpen = false"
        @open-palette="openTool('open-palette')"
        @open-shortcuts="openTool('open-shortcuts')"
      />
    </SheetContent>
  </Sheet>
</template>
