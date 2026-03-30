import type { ContractTemplateState } from '@/types/contract-template-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractTemplateStateFilterStore = defineStore('contractTemplateStateFilter', () => {
  const stateFilters: Ref<Set<ContractTemplateState>> = ref(new Set())

  function hasFilter(filter: ContractTemplateState) {
    return stateFilters.value.has( filter)
  }

  const hasFilters = computed(() => stateFilters.value.size > 0)

  function setFilter(filter: ContractTemplateState) {
    stateFilters.value.add(filter)
  }

  function removeFilter(filter: ContractTemplateState) {
    stateFilters.value.delete(filter)
  }

  function reset() {
    stateFilters.value.clear()
  }

  return { stateFilters, hasFilters: hasFilters, hasFilter, setFilter, removeFilter, reset }
})
