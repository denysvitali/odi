import { ref, onMounted, onUnmounted, watch } from 'vue'

export interface UseInfiniteScrollOptions {
  threshold?: number
  rootMargin?: string
}

export function useInfiniteScroll(
  callback: () => void,
  options: UseInfiniteScrollOptions = {}
) {
  const { threshold = 0.1, rootMargin = '100px' } = options

  const targetRef = ref<HTMLElement | null>(null)
  let observer: IntersectionObserver | null = null

  function disconnect() {
    if (observer) {
      observer.disconnect()
      observer = null
    }
  }

  function connect(el: HTMLElement) {
    disconnect()
    observer = new IntersectionObserver(
      (entries) => {
        const [entry] = entries
        if (entry.isIntersecting) {
          callback()
        }
      },
      {
        threshold,
        rootMargin
      }
    )
    observer.observe(el)
  }

  onMounted(() => {
    if (targetRef.value) {
      connect(targetRef.value)
    }
  })

  watch(targetRef, (el) => {
    if (el) {
      connect(el)
    } else {
      disconnect()
    }
  })

  onUnmounted(() => {
    disconnect()
  })

  return {
    targetRef
  }
}
