<script setup lang="ts">
import { ref } from 'vue'
import { Sparkles, ChevronDown, Check, Copy, AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useDocumentSummary } from '@/composables/useDocumentSummary'

interface Props {
  documentId: string
}

const props = defineProps<Props>()

const { summary, keyFacts, loading, error, summarize } = useDocumentSummary()

const expanded = ref(true)
const hasRun = ref(false)
const copiedIdx = ref<number | null>(null)

const onSummarize = async () => {
  hasRun.value = true
  expanded.value = true
  await summarize(props.documentId)
}

const copyFact = async (value: string, idx: number) => {
  try {
    await navigator.clipboard.writeText(value)
    copiedIdx.value = idx
    setTimeout(() => {
      if (copiedIdx.value === idx) {
        copiedIdx.value = null
      }
    }, 1500)
  } catch {
    // Clipboard unavailable (e.g. insecure context); silently ignore.
  }
}
</script>

<template>
  <div class="rounded-xl border bg-card">
    <button
      type="button"
      class="flex w-full items-center justify-between px-4 py-3 text-sm font-medium"
      @click="expanded = !expanded"
    >
      <span class="flex items-center gap-2">
        <Sparkles class="h-4 w-4 text-muted-foreground" />
        AI Summary
      </span>
      <ChevronDown
        :class="cn(
          'h-4 w-4 text-muted-foreground transition-transform',
          expanded && 'rotate-180'
        )"
      />
    </button>

    <div v-if="expanded" class="space-y-3 border-t px-4 py-3">
      <!-- Trigger -->
      <Button
        variant="outline"
        size="sm"
        class="gap-2"
        :disabled="loading"
        @click="onSummarize"
      >
        <Sparkles class="h-4 w-4" :class="loading && 'animate-pulse'" />
        {{ hasRun ? 'Regenerate summary' : 'Summarize' }}
      </Button>

      <!-- Loading state -->
      <div v-if="loading" class="flex items-center gap-2 text-xs text-muted-foreground">
        <div class="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
        Generating summary...
      </div>

      <!-- Error state -->
      <div
        v-else-if="error"
        class="flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive"
      >
        <AlertCircle class="h-4 w-4 shrink-0" />
        {{ error }}
      </div>

      <!-- Result -->
      <template v-else-if="hasRun">
        <p v-if="summary" class="text-sm leading-relaxed text-foreground">
          {{ summary }}
        </p>
        <p v-else class="text-sm text-muted-foreground">
          No summary available for this document.
        </p>

        <div v-if="keyFacts.length > 0" class="flex flex-wrap gap-1.5">
          <button
            v-for="(fact, idx) in keyFacts"
            :key="`${fact.label}-${idx}`"
            type="button"
            class="group inline-flex items-center gap-1.5 rounded-full border border-input bg-background px-2.5 py-1 text-xs transition-colors hover:bg-accent"
            :title="`Copy: ${fact.value}`"
            @click="copyFact(fact.value, idx)"
          >
            <span class="font-medium text-muted-foreground">{{ fact.label }}:</span>
            <span class="text-foreground">{{ fact.value }}</span>
            <Check v-if="copiedIdx === idx" class="h-3 w-3 text-green-600" />
            <Copy v-else class="h-3 w-3 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
          </button>
        </div>
      </template>
    </div>
  </div>
</template>
