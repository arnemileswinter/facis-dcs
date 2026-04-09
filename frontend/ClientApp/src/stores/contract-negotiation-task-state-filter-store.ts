import type { FilterStore } from '@/models/stores/filter-store'
import type { NegotiationTaskState } from '@/types/negotiation-task-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractNeogtiationTaskStateFilterStore = defineStore('contractNegotiationTaskStateFilter', () => {
  const stateFilters: Ref<Set<NegotiationTaskState>> = ref(new Set())

  const hasFilters = computed(() => stateFilters.value.size > 0)

  function hasFilter(filter: NegotiationTaskState) {
    return stateFilters.value.has(filter)
  }

  function setFilter(filter: NegotiationTaskState) {
    stateFilters.value.add(filter)
  }

  function removeFilter(filter: NegotiationTaskState) {
    stateFilters.value.delete(filter)
  }

  function reset() {
    stateFilters.value.clear()
  }

  return {
    stateFilters,
    hasFilters,
    hasFilter,
    setFilter,
    removeFilter,
    reset,
  } satisfies FilterStore<NegotiationTaskState>
})
