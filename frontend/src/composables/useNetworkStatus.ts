import { ref, onMounted, onUnmounted } from 'vue'

export function useNetworkStatus() {
  const online = ref(typeof navigator !== 'undefined' ? navigator.onLine : true)

  const update = () => {
    online.value = navigator.onLine
  }

  onMounted(() => {
    window.addEventListener('online', update)
    window.addEventListener('offline', update)
  })

  onUnmounted(() => {
    window.removeEventListener('online', update)
    window.removeEventListener('offline', update)
  })

  return { online }
}
