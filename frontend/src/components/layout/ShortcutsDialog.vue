<script setup lang="ts">
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

interface Props {
  open: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const isMac = computed(() =>
  typeof navigator !== 'undefined' && /Mac/i.test(navigator.platform)
)
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
  { keys: ['G then H'], description: 'Go to Home' },
  { keys: ['G then U'], description: 'Go to Upload' }
])

const close = () => emit('update:open', false)
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-200"
      leave-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="props.open"
        class="fixed inset-0 z-[60] flex items-center justify-center bg-background/70 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        aria-labelledby="shortcuts-title"
        @click.self="close"
        @keydown.esc="close"
      >
        <div class="w-full max-w-lg rounded-2xl border bg-card p-6 shadow-2xl">
          <div class="mb-4 flex items-center justify-between">
            <h2 id="shortcuts-title" class="text-lg font-semibold tracking-tight">
              Keyboard Shortcuts
            </h2>
            <Button variant="ghost" size="icon" aria-label="Close" @click="close">
              <X class="h-4 w-4" />
            </Button>
          </div>
          <ul class="space-y-2">
            <li
              v-for="s in shortcuts"
              :key="s.description"
              class="flex items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-muted"
            >
              <span class="text-muted-foreground">{{ s.description }}</span>
              <span class="flex gap-1">
                <kbd
                  v-for="k in s.keys"
                  :key="k"
                  class="rounded border bg-background px-1.5 py-0.5 text-xs font-medium"
                >
                  {{ k }}
                </kbd>
              </span>
            </li>
          </ul>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
