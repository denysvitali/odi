import { ref, computed } from 'vue'

export function useSelection() {
  const selected = ref<Set<string>>(new Set())
  const active = ref(false)

  const count = computed(() => selected.value.size)
  const isSelected = (id: string) => selected.value.has(id)

  const toggle = (id: string) => {
    const next = new Set(selected.value)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    selected.value = next
  }

  const selectAll = (ids: string[]) => {
    selected.value = new Set(ids)
  }

  const clear = () => {
    selected.value = new Set()
  }

  const setActive = (value: boolean) => {
    active.value = value
    if (!value) clear()
  }

  return { selected, active, count, isSelected, toggle, selectAll, clear, setActive }
}
