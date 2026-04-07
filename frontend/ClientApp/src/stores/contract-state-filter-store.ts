import type { ContractState } from '@/types/contract-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractStateFilterStore = defineStore('contractStateFilter', () => {
  const stateFilters: Ref<Set<ContractState>> = ref(new Set())

  function hasFilter(filter: ContractState) {
    return stateFilters.value.has( filter)
  }

  const hasFilters = computed(() => stateFilters.value.size > 0)

  function setFilter(filter: ContractState) {
    stateFilters.value.add(filter)
  }

  function removeFilter(filter: ContractState) {
    stateFilters.value.delete(filter)
  }

  function reset() {
    stateFilters.value.clear()
  }

  return { stateFilters, hasFilters: hasFilters, hasFilter, setFilter, removeFilter, reset }
})
