<script setup lang="ts">
import { computed, ref } from 'vue'
import { QrCode, CreditCard, Copy, Check } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { formatCurrency } from '@/lib/format'
import type { Barcode } from '@/types/documents'

interface Props {
  barcode: Barcode
}

const props = defineProps<Props>()
const copied = ref(false)

const isQrBill = computed(() => !!props.barcode.qrBill)

const formatIban = (iban?: string) => {
  if (!iban) return null
  return iban.replace(/(.{4})/g, '$1 ').trim()
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}
</script>

<template>
  <Card class="border-border/50">
    <CardHeader class="pb-3">
      <div class="flex items-center justify-between">
        <CardTitle class="flex items-center gap-2 text-sm font-medium">
          <QrCode v-if="isQrBill" class="h-4 w-4" aria-hidden="true" />
          <CreditCard v-else class="h-4 w-4" aria-hidden="true" />
          {{ isQrBill ? 'QR Bill' : 'Barcode' }}
        </CardTitle>
        <Badge v-if="barcode.qrBill?.referenceType" variant="outline" class="text-xs">
          {{ barcode.qrBill.referenceType }}
        </Badge>
      </div>
    </CardHeader>
    <CardContent class="space-y-4">
      <template v-if="barcode.qrBill">
        <div v-if="barcode.qrBill.amount !== undefined" class="space-y-1">
          <p class="text-xs text-muted-foreground">Amount</p>
          <p class="text-2xl font-semibold">
            {{ formatCurrency(barcode.qrBill.amount, barcode.qrBill.currency) }}
          </p>
        </div>

        <div v-if="barcode.qrBill.iban" class="space-y-1">
          <p class="text-xs text-muted-foreground">IBAN</p>
          <div class="flex items-center gap-2">
            <code class="rounded bg-muted px-2 py-1 font-mono text-sm">
              {{ formatIban(barcode.qrBill.iban) }}
            </code>
            <Button
              variant="ghost"
              size="icon"
              class="h-7 w-7"
              aria-label="Copy IBAN"
              @click="copyToClipboard(barcode.qrBill.iban!)"
            >
              <Check v-if="copied" class="h-3 w-3 text-green-500" aria-hidden="true" />
              <Copy v-else class="h-3 w-3" aria-hidden="true" />
            </Button>
          </div>
        </div>

        <div v-if="barcode.qrBill.reference" class="space-y-1">
          <p class="text-xs text-muted-foreground">Reference</p>
          <code class="block rounded bg-muted px-2 py-1 font-mono text-sm">
            {{ barcode.qrBill.reference }}
          </code>
        </div>

        <Separator v-if="barcode.qrBill.creditor || barcode.qrBill.debtor" />

        <div v-if="barcode.qrBill.creditor" class="space-y-1">
          <p class="text-xs text-muted-foreground">Creditor</p>
          <div class="text-sm">
            <p v-if="barcode.qrBill.creditor.name" class="font-medium">
              {{ barcode.qrBill.creditor.name }}
            </p>
            <p v-if="barcode.qrBill.creditor.address" class="text-muted-foreground">
              {{ barcode.qrBill.creditor.address }}
            </p>
            <p
              v-if="barcode.qrBill.creditor.postalCode || barcode.qrBill.creditor.city"
              class="text-muted-foreground"
            >
              {{ barcode.qrBill.creditor.postalCode }} {{ barcode.qrBill.creditor.city }}
            </p>
          </div>
        </div>

        <div v-if="barcode.qrBill.debtor" class="space-y-1">
          <p class="text-xs text-muted-foreground">Debtor</p>
          <div class="text-sm">
            <p v-if="barcode.qrBill.debtor.name" class="font-medium">
              {{ barcode.qrBill.debtor.name }}
            </p>
            <p v-if="barcode.qrBill.debtor.address" class="text-muted-foreground">
              {{ barcode.qrBill.debtor.address }}
            </p>
            <p
              v-if="barcode.qrBill.debtor.postalCode || barcode.qrBill.debtor.city"
              class="text-muted-foreground"
            >
              {{ barcode.qrBill.debtor.postalCode }} {{ barcode.qrBill.debtor.city }}
            </p>
          </div>
        </div>

        <div v-if="barcode.qrBill.additionalInformation" class="space-y-1">
          <p class="text-xs text-muted-foreground">Additional Information</p>
          <p class="text-sm">{{ barcode.qrBill.additionalInformation }}</p>
        </div>
      </template>

      <template v-else>
        <div class="space-y-1">
          <p class="text-xs text-muted-foreground">Content</p>
          <code class="block break-all rounded bg-muted px-2 py-1 font-mono text-sm">
            {{ barcode.text }}
          </code>
        </div>
      </template>
    </CardContent>
  </Card>
</template>
