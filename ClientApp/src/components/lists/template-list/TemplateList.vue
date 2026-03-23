<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import { useContractTemplateStateFilterStore } from '@/stores/contract-template-state-filter-store'
import { storeToRefs } from 'pinia'
import { computed, ref, type Ref } from 'vue'
import ListSearch from '../ListSearch.vue'
import ListSort from '../ListSort.vue'
import TemplateListItem from './TemplateListItem.vue'

const props = defineProps<{
  items: PartialContractTemplate[]
}>()

const sorter = new Map([
  ['created_at', 'Creation date'],
  ['state', 'Status'],
])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const stateFilterStore = useContractTemplateStateFilterStore()
const { stateFilters } = storeToRefs(stateFilterStore)

function valueToComparable(value: unknown) {
  if (typeof value === 'number') return value
  if (value instanceof Date) return value.getTime()
  if (typeof value === 'string') {
    const dateTime = new Date(value).getTime()
    return Number.isNaN(dateTime) ? value : dateTime
  }
  return undefined
}

const searchedItems: Ref<PartialContractTemplate[]> = ref(props.items)

const sortedItems = computed(() => {
  if (!sorter.has(sortBy.value)) {
    return searchedItems.value
  }
  return searchedItems.value.slice().sort((a, b) => {
    let aSortValue = a[sortBy.value as keyof PartialContractTemplate]
    let bSortValue = b[sortBy.value as keyof PartialContractTemplate]
    const aValue = valueToComparable(aSortValue)
    const bValue = valueToComparable(bSortValue)
    if (!aValue && !bValue) return 0
    if (!aValue) return sortOrder.value
    if (!bValue) return sortOrder.value * -1

    let result: number
    if (typeof aValue === 'number' && typeof bValue === 'number') {
      result = aValue > bValue ? 1 : -1
    } else {
      result = String(aValue) > String(bValue) ? 1 : -1
    }
    return sortOrder.value * result
  })
})

const filteredItems = computed(() => {
  const filters = stateFilters.value
  if (filters.size > 0) {
    return sortedItems.value.filter((item) => filters.has(item.state))
  }
  return sortedItems.value
})

function applySearchResult(searchResult: PartialContractTemplate[]) {
  searchedItems.value = searchResult
}
</script>

<template>
  <ul class="list">
    <li class="tracking-wide px-4 flex justify-between">
      <ListSearch :items="items" class="grow" @search-result="applySearchResult" />
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <TemplateListItem
      v-for="item in filteredItems"
      :key="`${item.did},${item.document_number},${item.version}`"
      :item="item"
    />
  </ul>
</template>
