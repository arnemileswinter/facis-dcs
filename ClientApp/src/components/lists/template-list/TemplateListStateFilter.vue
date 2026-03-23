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

function setFilter(stateFilter: ContractTemplateStateFilter) {
  if (stateFilters.value.has(stateFilter)) {
    stateFilterStore.removeFilter(stateFilter)
    showAll.value = !hasFilters.value
  } else {
    stateFilterStore.setFilter(stateFilter)
    showAll.value = false
  }
}

function isSelected(type: ContractTemplateStateFilter) {
  return stateFilters.value.has(type)
}
</script>

<template>
  <div>
    <div class="collapse collapse-arrow bg-base-300 border-base-300 border hover:bg-base-100">
      <input type="checkbox" />
      <div class="collapse-title text-sm">Contract Template</div>
      <div class="collapse-content">
        <ul class="list rounded-box shadow-sm">
          <li
            v-for="filter in shownFilters"
            :key="filter"
            class="list-row flex justify-between w-full cursor-pointer py-2 hover:bg-base-200"
            @click="setFilter(filter)"
          >
            <label class="label flex-1 cursor-pointer" :class="{ 'font-bold': isSelected(filter) }">{{ filter }}</label>
          </li>
          <li class="text-sm opacity-60 px-4">
            <label v-if="hasFilters" class="link" @click="showAll = !showAll">
              <div v-if="!showAll">See all</div>
              <div v-else>See less</div>
            </label>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
