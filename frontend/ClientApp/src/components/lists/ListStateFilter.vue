<script setup lang="ts" generic="T extends string[]">
import { useContractStateFilterStore } from '@/stores/contract-state-filter-store'
import { useContractTemplateApprovalTaskStateFilterStore } from '@/stores/contract-template-approval-task-state-filter-store'
import { useContractTemplateReviewTaskStateFilterStore } from '@/stores/contract-template-review-task-state-filter-store'
import { useContractTemplateStateFilterStore } from '@/stores/contract-template-state-filter-store'
import { computed, ref } from 'vue'

const props = defineProps<{
  filters: T
  label: string
  storeType: 'templates' | 'contracts' | 'reviewTasks' | 'approvalTasks'
}>()

const storeMap = {
  templates: useContractTemplateStateFilterStore,
  contracts: useContractStateFilterStore,
  reviewTasks: useContractTemplateReviewTaskStateFilterStore,
  approvalTasks: useContractTemplateApprovalTaskStateFilterStore,
} as const

const filterStore = storeMap[props.storeType]()

const showAll = ref(true)

const activeFilters = computed(() => {
  return props.filters.filter((filter) => filterStore.hasFilter(filter as any))
})

const inactiveFilters = computed(() => {
  return showAll.value ? props.filters.filter((filter) => !filterStore.hasFilter(filter as any)) : []
})

const shownFilters = computed(() => {
  return [...activeFilters.value, ...inactiveFilters.value]
})

const hasFilters = computed(() => {
  return activeFilters.value.length > 0
})

const setFilter = (stateFilter: T[number]) => {
  if (filterStore.hasFilter(stateFilter as any)) {
    filterStore.removeFilter(stateFilter as any)
    showAll.value = !hasFilters.value
  } else {
    filterStore.setFilter(stateFilter as any)
    showAll.value = false
  }
}

const isSelected = (type: T[number]) => {
  return filterStore.hasFilter(type as any)
}
</script>

<template>
  <button id="popover-btn" popovertarget="filter-popover" class="select select-secondary w-fit gap-2 m-2">
    Filter
  </button>
  <ul id="filter-popover" popover class="dropdown menu rounded-box rounded-md bg-base-300 shadow-sm">
    <li>
      <label class="label">{{ label }}</label>
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
