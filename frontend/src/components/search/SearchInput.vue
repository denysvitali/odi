<script setup lang="ts">
import { ref } from 'vue'
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const modelValue = defineModel<string>()

const emit = defineEmits<{
  submit: []
}>()

const inputRef = ref<HTMLInputElement>()

const handleSubmit = () => {
  if (modelValue.value?.trim()) emit('submit')
}

const clearSearch = () => {
  modelValue.value = ''
  inputRef.value?.focus()
}
</script>

<template>
  <div class="relative mx-auto w-full max-w-2xl">
    <div class="relative">
      <Search class="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
      <Input
        ref="inputRef"
        v-model="modelValue"
        type="search"
        placeholder="Search documents…"
        aria-label="Search documents"
        data-global-search
        class="h-14 w-full rounded-2xl border-border/50 bg-secondary/50 pl-12 pr-24 text-base shadow-sm transition-all duration-200 focus:bg-background focus:shadow-md"
        @keyup.enter="handleSubmit"
      />
      <div class="absolute right-3 top-1/2 flex -translate-y-1/2 items-center gap-2">
        <Button
          v-if="modelValue"
          variant="ghost"
          size="icon"
          class="h-8 w-8 text-muted-foreground hover:text-foreground"
          aria-label="Clear search"
          @click="clearSearch"
        >
          <X class="h-4 w-4" aria-hidden="true" />
        </Button>
        <kbd class="hidden h-8 items-center gap-1 rounded-lg border bg-muted px-2.5 text-xs font-medium text-muted-foreground sm:inline-flex">
          <span class="text-xs">/</span>
        </kbd>
      </div>
    </div>
  </div>
</template>
