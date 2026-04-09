<script setup lang="ts">
import type { ContractNegotiationTask } from '@/models/contract/contract-negotiation-task'
import { useContractNeogtiationTaskStateFilterStore } from '@/stores/contract-negotiation-task-state-filter-store'
import { useContractsStore } from '@/stores/contracts-store'
import { negotiationTaskStates } from '@/types/negotiation-task-state'
import { toComparableValue } from '@/utils/comparison'
import { toProperCase } from '@/utils/string'
import { computed, onUnmounted, ref, type Ref } from 'vue'
import ListSort from '../ListSort.vue'
import ListStateFilter from '../ListStateFilter.vue'
import TaskListSearch from './TaskListSearch.vue'

const props = defineProps<{
  items: ContractNegotiationTask[]
}>()

const contractsStore = useContractsStore()
const stateFilterStore = useContractNeogtiationTaskStateFilterStore()

const sorter = new Map([
  ['created_at', 'Creation date'],
  ['state', 'Task state'],
])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const searchFilteredItems: Ref<ContractNegotiationTask[]> = ref([])

const searchedItems = computed(() => {
  return searchFilteredItems.value.length > 0 ? searchFilteredItems.value : props.items
})

const sortedItems = computed(() => {
  if (!sorter.has(sortBy.value)) {
    return searchedItems.value
  }
  return searchedItems.value.slice().sort((taskA, taskB) => {
    const aSortValue = taskA[sortBy.value as keyof ContractNegotiationTask]
    const bSortValue = taskB[sortBy.value as keyof ContractNegotiationTask]
    const aValue = toComparableValue(aSortValue)
    const bValue = toComparableValue(bSortValue)
    if (!aValue && !bValue) return 0
    if (!aValue) return sortOrder.value
    if (!bValue) return sortOrder.value * -1

    let result: number
    if (typeof aValue === 'number' && typeof bValue === 'number') {
      result = Math.sign(bValue - aValue)
    } else {
      result = String(aValue).localeCompare(String(bValue))
    }
    return sortOrder.value * result
  })
})

const filteredItems = computed(() => {
  if (stateFilterStore.hasFilters) {
    return sortedItems.value.filter((item) => stateFilterStore.hasFilter(item.state))
  }
  return sortedItems.value
})

const getContractName = (item: ContractNegotiationTask) => {
  return contractsStore.contracts.find((contract) => contract.did === item.did)?.name ?? 'Nameless Contract'
}

const applySearchResult = (searchResult: ContractNegotiationTask[]) => {
  searchFilteredItems.value = props.items.filter((task) =>
    searchResult.map((template) => template.did).includes(task.did),
  )
}

onUnmounted(() => stateFilterStore.reset())
</script>

<template>
  <ul class="list">
    <li class="tracking-wide w-full px-4 flex justify-end flex-col sm:flex-row">
      <ListStateFilter label="Negotiation Task" :filters="negotiationTaskStates" store-type="negotiationTasks" />
      <TaskListSearch class="flex-1" :items="items" @search-result="applySearchResult" />
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <li v-for="item in filteredItems" class="list-row">
      <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300">
        <div class="card-body">
          <h2 class="card-title flex-wrap justify-between">
            <div>Negotiation Task for Contract: {{ getContractName(item) }}</div>
            <div class="flex-1"></div>
            <div class="badge badge-accent">{{ toProperCase(item.type) }} Task</div>
            <div class="badge badge-secondary">{{ item.state }}</div>
          </h2>
          <div class="flex justify-between">
            <div v-if="item.contract_version">Version: {{ item.contract_version }}</div>
          </div>
          <div class="flex justify-between">
            <div>Creation date: {{ new Date(item.created_at).toLocaleDateString() }}</div>
            <div class="card-actions justify-end">
              <RouterLink
                to="#"
                class="btn btn-sm btn-primary rounded-box"
              >
                View
              </RouterLink>
            </div>
          </div>
        </div>
      </div>
    </li>
  </ul>
</template>
