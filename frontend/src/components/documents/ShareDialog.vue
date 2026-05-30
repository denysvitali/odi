<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Link2, Copy, Check, AlertCircle, Loader2 } from 'lucide-vue-next'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useShares } from '@/composables/useShares'
import { useClipboard } from '@/composables/useClipboard'

interface Props {
  /** The OpenSearch document id, e.g. "<scanID>_<sequenceId>". Optional when
   *  scanID + sequenceId are passed explicitly. */
  docId?: string
  scanID?: string
  sequenceId?: number
  open: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { create, creating, error, shareUrl } = useShares()
const { copied, copy } = useClipboard()

const expiresInHours = ref('24')
const maxViews = ref('0')
const passphrase = ref('')
const generatedToken = ref<string | null>(null)

// Resolve scanID / sequenceId from either the explicit props or the docId.
const resolved = computed<{ scanID: string; sequenceId: number } | null>(() => {
  if (props.scanID && props.sequenceId !== undefined) {
    return { scanID: props.scanID, sequenceId: props.sequenceId }
  }
  if (props.docId) {
    const idx = props.docId.lastIndexOf('_')
    if (idx > 0) {
      const scanID = props.docId.slice(0, idx)
      const seq = Number(props.docId.slice(idx + 1))
      if (scanID && Number.isFinite(seq)) return { scanID, sequenceId: seq }
    }
  }
  return null
})

const generatedUrl = computed(() => (generatedToken.value ? shareUrl(generatedToken.value) : ''))

function reset() {
  expiresInHours.value = '24'
  maxViews.value = '0'
  passphrase.value = ''
  generatedToken.value = null
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) reset()
  }
)

async function generate() {
  const target = resolved.value
  if (!target) return
  const result = await create({
    scanID: target.scanID,
    sequenceID: target.sequenceId,
    expiresInHours: Number(expiresInHours.value) > 0 ? Number(expiresInHours.value) : 24,
    maxViews: Number(maxViews.value) > 0 ? Number(maxViews.value) : 0,
    passphrase: passphrase.value || undefined
  })
  if (result) {
    generatedToken.value = result.token
  }
}

async function copyUrl() {
  if (generatedUrl.value) await copy(generatedUrl.value, 'share link')
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <Sheet :open="open" @update:open="(v) => emit('update:open', v)">
    <SheetContent side="right" class="w-full sm:max-w-md">
      <SheetHeader>
        <SheetTitle class="flex items-center gap-2">
          <Link2 class="h-5 w-5" aria-hidden="true" />
          Share document
        </SheetTitle>
        <SheetDescription>
          Create a secure, expiring link. Anyone with the link can view this page until it expires or is revoked.
        </SheetDescription>
      </SheetHeader>

      <div class="mt-6 space-y-5">
        <div v-if="!resolved" class="flex items-center gap-2 rounded-lg border border-red-500/40 p-3 text-sm text-red-600">
          <AlertCircle class="h-4 w-4" aria-hidden="true" />
          No document selected to share.
        </div>

        <template v-else>
          <div v-if="!generatedToken" class="space-y-5">
            <div class="space-y-2">
              <label class="text-sm font-medium" for="share-expiry">Expires in (hours)</label>
              <Input id="share-expiry" v-model="expiresInHours" type="number" min="1" placeholder="24" />
            </div>

            <div class="space-y-2">
              <label class="text-sm font-medium" for="share-maxviews">Max views (0 = unlimited)</label>
              <Input id="share-maxviews" v-model="maxViews" type="number" min="0" placeholder="0" />
            </div>

            <div class="space-y-2">
              <label class="text-sm font-medium" for="share-pass">Passphrase (optional)</label>
              <Input id="share-pass" v-model="passphrase" type="password" placeholder="Leave empty for no passphrase" />
              <p class="text-xs text-muted-foreground">Recipients must enter this passphrase to view the document.</p>
            </div>

            <div v-if="error" class="flex items-center gap-2 rounded-lg border border-red-500/40 p-3 text-sm text-red-600">
              <AlertCircle class="h-4 w-4" aria-hidden="true" />
              {{ error }}
            </div>
          </div>

          <div v-else class="space-y-3">
            <p class="text-sm text-muted-foreground">Share link created. Copy it and send it to your recipient.</p>
            <div class="flex items-center gap-2">
              <Input :model-value="generatedUrl" readonly class="font-mono text-xs" />
              <Button variant="outline" size="icon" aria-label="Copy share link" @click="copyUrl">
                <Check v-if="copied" class="h-4 w-4 text-green-600" aria-hidden="true" />
                <Copy v-else class="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
            <p v-if="passphrase" class="text-xs text-muted-foreground">
              Remember to share the passphrase separately — it is not included in the link.
            </p>
          </div>
        </template>
      </div>

      <SheetFooter class="mt-6">
        <template v-if="resolved && !generatedToken">
          <Button variant="outline" @click="close">Cancel</Button>
          <Button :disabled="creating" @click="generate">
            <Loader2 v-if="creating" class="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
            <Link2 v-else class="mr-2 h-4 w-4" aria-hidden="true" />
            Create link
          </Button>
        </template>
        <template v-else>
          <Button @click="close">Done</Button>
        </template>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
