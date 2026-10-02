<script setup lang="ts">
import { useRoute } from 'vue-router'
import {
  Archive,
  LayoutDashboard,
  Files,
  Star,
  Upload,
  MessageSquare,
  Share2,
  Settings2,
  ArrowUpRight,
  Keyboard,
  Leaf
} from 'lucide-vue-next'
import { useFavorites } from '@/composables/useFavorites'

const emit = defineEmits<{ 'open-palette': []; 'open-shortcuts': []; navigate: [] }>()
const route = useRoute()
const { count } = useFavorites()
const groups = [
  {
    title: 'Workspace',
    items: [
      { name: 'Overview', path: '/', icon: LayoutDashboard },
      { name: 'All documents', path: '/documents', icon: Files },
      { name: 'Favorites', path: '/favorites', icon: Star }
    ]
  },
  {
    title: 'Tools',
    items: [
      { name: 'Ask your archive', path: '/chat', icon: MessageSquare },
      { name: 'Shared links', path: '/shares', icon: Share2 },
      { name: 'Administration', path: '/admin', icon: Settings2 }
    ]
  }
]
const isActive = (path: string) => (path === '/' ? route.path === '/' : route.path.startsWith(path))
</script>

<template>
  <aside class="archive-sidebar" aria-label="Archive navigation">
    <RouterLink to="/" class="archive-brand" @click="emit('navigate')">
      <span class="brand-symbol"><Archive :size="21" aria-hidden="true" /></span>
      <span>odi<span class="brand-dot">.</span><small>YOUR DOCUMENT WORKSPACE</small></span>
    </RouterLink>
    <div class="sidebar-workspace">
      <span class="workspace-avatar">P</span>
      <div>Personal archive<small>Your documents, organized.</small></div>
    </div>
    <nav class="sidebar-nav" aria-label="Primary">
      <div v-for="group in groups" :key="group.title" class="nav-group">
        <p class="eyebrow">{{ group.title }}</p>
        <RouterLink
          v-for="item in group.items"
          :key="item.path"
          :to="item.path"
          class="sidebar-link"
          :class="{ 'is-active': isActive(item.path) }"
          :aria-current="isActive(item.path) ? 'page' : undefined"
          @click="emit('navigate')"
        >
          <component :is="item.icon" :size="18" aria-hidden="true" />
          {{ item.name }}
          <span v-if="item.path === '/favorites' && count" class="nav-count">{{ count }}</span>
        </RouterLink>
      </div>
    </nav>
    <div class="sidebar-bottom">
      <RouterLink to="/upload" class="sidebar-upload" @click="emit('navigate')"
        ><Upload :size="17" aria-hidden="true" /> Add documents
        <ArrowUpRight :size="16" class="ml-auto" aria-hidden="true"
      /></RouterLink>
      <div class="privacy-note">
        <Leaf :size="17" aria-hidden="true" />
        <div>A little less paper.<small>A little more peace of mind.</small></div>
      </div>
      <button type="button" class="sidebar-shortcut" @click="emit('open-shortcuts')">
        <Keyboard :size="16" aria-hidden="true" /> Keyboard shortcuts <kbd>?</kbd>
      </button>
      <p class="sidebar-version">OPEN DOCUMENT INDEXER</p>
    </div>
  </aside>
</template>
