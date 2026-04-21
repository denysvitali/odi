<script setup lang="ts">
import { computed, ref } from 'vue'
import { ExternalLink, Building2, MapPin, Tag, Copy, Check } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { Company } from '@/types/documents'

interface Props {
  company: Company
}

const props = defineProps<Props>()
const copied = ref(false)

const zefixUrl = computed(() => {
  if (!props.company.uri) return null
  return `https://www.zefix.ch/de/search/entity/list?name=${encodeURIComponent(props.company.name)}`
})

const isPrimary = computed(() => props.company.type === 'primary')

const addressLine = computed(() => props.company.address || props.company.locality || '')

const copyAddress = async () => {
  if (!addressLine.value) return
  try {
    await navigator.clipboard.writeText(addressLine.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {}
}
</script>

<template>
  <Card class="border-border/50">
    <CardContent class="space-y-3 p-4">
      <div class="flex items-start justify-between gap-2">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span class="font-medium">{{ company.name }}</span>
            <Badge v-if="isPrimary" variant="secondary" class="text-xs">Primary</Badge>
          </div>
          <p v-if="company.legalName && company.legalName !== company.name" class="text-sm text-muted-foreground">
            {{ company.legalName }}
          </p>
        </div>
      </div>

      <div v-if="addressLine" class="flex items-start gap-2 text-sm text-muted-foreground">
        <MapPin class="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span class="flex-1">{{ addressLine }}</span>
        <button
          type="button"
          class="rounded-md p-1 hover:bg-muted"
          :aria-label="copied ? 'Copied' : 'Copy address'"
          @click="copyAddress"
        >
          <Check v-if="copied" class="h-3.5 w-3.5 text-green-500" />
          <Copy v-else class="h-3.5 w-3.5" />
        </button>
      </div>

      <div v-if="company.type && !isPrimary" class="flex items-center gap-2 text-sm text-muted-foreground">
        <Tag class="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{{ company.type }}</span>
      </div>

      <Button
        v-if="zefixUrl"
        variant="outline"
        size="sm"
        class="mt-2 w-full"
        as="a"
        :href="zefixUrl"
        target="_blank"
        rel="noopener noreferrer"
      >
        <Building2 class="mr-2 h-4 w-4" aria-hidden="true" />
        View in Zefix
        <ExternalLink class="ml-auto h-3 w-3" aria-hidden="true" />
      </Button>
    </CardContent>
  </Card>
</template>
