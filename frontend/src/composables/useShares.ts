import { ref } from 'vue'
import { getApiUrl } from '@/lib/config'
import { api, type CreateShareOptions, type CreateShareResult, type Share } from '@/api/client'

/**
 * useShares — create, list, and revoke secure expiring share links.
 *
 * Backend API contract (implemented in src/api/client.ts by the wiring
 * engineer):
 *   api.createShare(opts: CreateShareOptions): Promise<CreateShareResult>
 *   api.listShares(): Promise<Share[]>
 *   api.revokeShare(token: string): Promise<void>
 *
 * shareUrl(token) builds the PUBLIC, unauthenticated link a recipient opens:
 *   `${getApiUrl()}/share/${token}`
 */
export function useShares() {
  const shares = ref<Share[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const error = ref<string | null>(null)

  function shareUrl(token: string): string {
    return `${getApiUrl()}/share/${encodeURIComponent(token)}`
  }

  async function list(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      shares.value = await api.listShares()
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Unable to load shares'
    } finally {
      loading.value = false
    }
  }

  async function create(opts: CreateShareOptions): Promise<CreateShareResult | null> {
    creating.value = true
    error.value = null
    try {
      const result = await api.createShare(opts)
      return result
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Unable to create share link'
      return null
    } finally {
      creating.value = false
    }
  }

  async function revoke(token: string): Promise<boolean> {
    error.value = null
    try {
      await api.revokeShare(token)
      shares.value = shares.value.filter((s) => s.token !== token)
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Unable to revoke share link'
      return false
    }
  }

  return { shares, loading, creating, error, list, create, revoke, shareUrl }
}
