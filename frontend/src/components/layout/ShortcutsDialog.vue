<script setup lang="ts">
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose
} from 'reka-ui'

interface Props {
  open: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const isMac = computed(() => typeof navigator !== 'undefined' && /Mac/i.test(navigator.platform))
const mod = computed(() => (isMac.value ? '⌘' : 'Ctrl'))

const shortcuts = computed(() => [
  { keys: [mod.value + '+K'], description: 'Open search' },
  { keys: ['/'], description: 'Focus search input' },
  { keys: ['Esc'], description: 'Close dialogs / clear selection' },
  { keys: ['?'], description: 'Show this help' },
  { keys: ['←', '→', '↑', '↓'], description: 'Navigate document grid' },
  { keys: ['Enter'], description: 'Open focused document' },
  { keys: ['S'], description: 'Star / unstar focused document' },
  { keys: ['G then D'], description: 'Go to Documents' },
  { keys: ['G then H'], description: 'Go to Overview' },
  { keys: ['G then U'], description: 'Go to Upload' }
])
</script>

<template>
  <DialogRoot :open="props.open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[60] bg-black/40" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-[60] max-h-[calc(100dvh-32px)] w-[calc(100vw-32px)] max-w-lg -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border bg-card p-6 shadow-xl"
      >
        <div class="mb-4 flex items-center justify-between">
          <DialogTitle class="text-lg font-semibold tracking-tight">Keyboard Shortcuts</DialogTitle
          ><DialogClose as-child
            ><Button type="button" variant="ghost" size="icon" aria-label="Close keyboard shortcuts"
              ><X :size="18" aria-hidden="true" /></Button
          ></DialogClose>
        </div>
        <DialogDescription class="sr-only"
          >Shortcuts for navigating your archive and working with documents.</DialogDescription
        >
        <ul class="space-y-2">
          <li
            v-for="shortcut in shortcuts"
            :key="shortcut.description"
            class="flex items-center justify-between gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-muted"
          >
            <span class="text-muted-foreground">{{ shortcut.description }}</span
            ><span class="flex shrink-0 gap-1"
              ><kbd
                v-for="key in shortcut.keys"
                :key="key"
                class="rounded border bg-background px-1.5 py-0.5 text-xs font-medium"
                >{{ key }}</kbd
              ></span
            >
          </li>
        </ul>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
