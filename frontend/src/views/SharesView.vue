<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { AlertCircle, Copy, Check, Link2, Loader2, RefreshCw, Trash2, Lock } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useShares } from '@/composables/useShares'
import { useClipboard } from '@/composables/useClipboard'

const { shares, loading, error, list, revoke, shareUrl } = useShares()
const { copied, copy } = useClipboard()

const hasShares = computed(() => shares.value.length > 0)

function formatExpiry(unixSeconds: number): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(unixSeconds * 1000))
}

async function copyLink(token: string) {
  await copy(shareUrl(token), 'share link')
}

async function revokeShare(token: string) {
  await revoke(token)
}

onMounted(() => {
  void list()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">Shares</h1>
        <p class="text-muted-foreground">Manage active secure share links.</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" :disabled="loading" @click="list">
          <RefreshCw class="mr-2 h-4 w-4" :class="{ 'animate-spin': loading }" aria-hidden="true" />
          Refresh
        </Button>
      </div>
    </div>

    <Card v-if="error" class="border-red-500/50">
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-red-600">
          <AlertCircle class="h-5 w-5" aria-hidden="true" />
          Share action failed
        </CardTitle>
        <CardDescription>{{ error }}</CardDescription>
      </CardHeader>
    </Card>

    <Card v-if="loading && !hasShares">
      <CardContent class="flex items-center gap-2 py-8 text-muted-foreground">
        <Loader2 class="h-5 w-5 animate-spin" aria-hidden="true" />
        Loading shares...
      </CardContent>
    </Card>

    <Card v-else-if="!hasShares">
      <CardContent class="flex flex-col items-center gap-2 py-12 text-center text-muted-foreground">
        <Link2 class="h-8 w-8" aria-hidden="true" />
        <p>No active share links.</p>
        <p class="text-sm">Create a share link from a document's actions to see it here.</p>
      </CardContent>
    </Card>

    <div v-else class="space-y-3">
      <Card v-for="share in shares" :key="share.token">
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Link2 class="h-4 w-4" aria-hidden="true" />
            <span class="font-mono">{{ share.scanID }}_{{ share.sequenceID }}</span>
            <Badge v-if="share.hasPassphrase" variant="secondary" class="ml-1">
              <Lock class="mr-1 h-3 w-3" aria-hidden="true" />
              Passphrase
            </Badge>
          </CardTitle>
          <CardDescription>
            Expires {{ formatExpiry(share.expiresAt) }}
            <template v-if="share.maxViews > 0"> · {{ share.viewCount }} / {{ share.maxViews }} views</template>
            <template v-else> · {{ share.viewCount }} views</template>
          </CardDescription>
        </CardHeader>
        <CardContent class="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" @click="copyLink(share.token)">
            <Check v-if="copied" class="mr-2 h-4 w-4 text-green-600" aria-hidden="true" />
            <Copy v-else class="mr-2 h-4 w-4" aria-hidden="true" />
            Copy link
          </Button>
          <Button variant="outline" size="sm" class="text-red-600 hover:text-red-700" @click="revokeShare(share.token)">
            <Trash2 class="mr-2 h-4 w-4" aria-hidden="true" />
            Revoke
          </Button>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
