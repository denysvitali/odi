<script setup lang="ts">
import { computed, ref } from 'vue'
import { Search, X, Sparkles } from 'lucide-vue-next'
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

const syntaxHints = [
  { token: 'AND', label: 'requires both terms' },
  { token: 'OR', label: 'matches either term' },
  { token: 'NOT', label: 'excludes a term' },
  { token: '"quotes"', label: 'exact phrase' }
]

const activeSyntax = computed(() => {
  const query = modelValue.value || ''

  return syntaxHints.filter((hint) => {
    if (hint.token === '"quotes"') return /"[^"]+"/.test(query)
    return new RegExp(`\\b${hint.token}\\b`, 'i').test(query)
  })
})
</script>

<template>
  <div class="relative mx-auto w-full max-w-2xl">
    <div
      class="group relative rounded-[1.35rem] bg-gradient-to-r from-primary/70 via-sky-400/60 to-apple-purple/70 p-px shadow-[0_18px_60px_rgba(0,0,0,0.28)] transition-all duration-200 focus-within:shadow-[0_22px_80px_rgba(0,122,255,0.24)]"
    >
      <div class="relative rounded-[1.3rem] bg-background/95 backdrop-blur">
        <Search
          class="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground transition-colors group-focus-within:text-primary"
          aria-hidden="true"
        />
      <Input
        ref="inputRef"
        v-model="modelValue"
        type="text"
        placeholder="Search documents, e.g. invoice AND swisscom"
        aria-label="Search documents"
        data-global-search
          class="h-14 w-full rounded-[1.3rem] border-0 bg-secondary/40 pl-12 pr-14 text-base shadow-none transition-all duration-200 placeholder:text-muted-foreground/70 focus:bg-background/80 focus-visible:ring-0 focus-visible:ring-offset-0"
        @keyup.enter="handleSubmit"
      />
      <Button
        v-if="modelValue"
        variant="ghost"
        size="icon"
          class="absolute right-3 top-1/2 h-8 w-8 -translate-y-1/2 rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
        aria-label="Clear search"
        @click="clearSearch"
      >
        <X class="h-4 w-4" aria-hidden="true" />
      </Button>
      </div>
    </div>

    <div class="mt-3 flex flex-wrap items-center justify-center gap-2 text-xs text-muted-foreground">
      <span class="inline-flex items-center gap-1.5 rounded-full border border-border/60 bg-background/70 px-2.5 py-1">
        <Sparkles class="h-3.5 w-3.5 text-primary" aria-hidden="true" />
        OpenSearch syntax
      </span>
      <span
        v-for="hint in syntaxHints"
        :key="hint.token"
        class="rounded-full border px-2.5 py-1 transition-all"
        :class="activeSyntax.includes(hint)
          ? 'border-primary/60 bg-primary/15 text-primary shadow-sm'
          : 'border-border/50 bg-secondary/40 text-muted-foreground'"
        :title="hint.label"
      >
        {{ hint.token }}
      </span>
    </div>
  </div>
</template>
