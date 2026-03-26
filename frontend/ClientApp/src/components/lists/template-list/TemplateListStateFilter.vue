<script setup lang="ts">
import { useContractTemplateStateFilterStore } from '@/stores/contract-template-state-filter-store'
import {
  contractTemplateStates,
  type ContractTemplateState as ContractTemplateStateFilter,
} from '@/types/contract-template-state'
import { storeToRefs } from 'pinia'
import { computed, ref, type Ref } from 'vue'

const filters: Ref<ContractTemplateStateFilter[]> = ref(contractTemplateStates)
const stateFilterStore = useContractTemplateStateFilterStore()
const { stateFilters } = storeToRefs(stateFilterStore)

const showAll = ref(true)

const activeFilters = computed(() => {
  return filters.value.filter((filter) => stateFilters.value.has(filter))
})

const inactiveFilters = computed(() => {
  return showAll.value ? filters.value.filter((filter) => !stateFilters.value.has(filter)) : []
})

const shownFilters = computed(() => {
  return [...activeFilters.value, ...inactiveFilters.value]
})

const hasFilters = computed(() => {
  return activeFilters.value.length > 0
})

const setFilter = (stateFilter: ContractTemplateStateFilter) => {
  if (stateFilters.value.has(stateFilter)) {
    stateFilterStore.removeFilter(stateFilter)
    showAll.value = !hasFilters.value
  } else {
    stateFilterStore.setFilter(stateFilter)
    showAll.value = false
  }
}

const isSelected = (type: ContractTemplateStateFilter) => {
  return stateFilters.value.has(type)
}
</script>

<template>
  <button id="popover-btn" popovertarget="filter-popover" class="select select-secondary w-fit gap-2 m-2">
    Filter
  </button>
  <ul id="filter-popover" popover class="dropdown menu rounded-box rounded-md bg-base-300 shadow-sm">
    <li>
      <label class="label">Contract Template</label>
      <ul>
        <li
          v-for="filter in shownFilters"
          :key="filter"
          class="flex justify-between transition-colors"
          @click="setFilter(filter)"
        >
          <label class="label flex-1" :class="{ 'font-bold': isSelected(filter) }">{{ filter }}</label>
        </li>
        <li v-if="hasFilters" class="text-sm w-full opacity-60 px-4 py-2 border-t border-base-300">
          <label class="link cursor-pointer" @click="showAll = !showAll">
            <div v-if="!showAll">See all</div>
            <div v-else>See less</div>
          </label>
        </li>
      </ul>
    </li>
  </ul>
</template>

<style scoped>
#popover-btn {
  anchor-name: --anchor-filter-popover;
}

#filter-popover {
  position-anchor: --anchor-filter-popover;
}
</style>
