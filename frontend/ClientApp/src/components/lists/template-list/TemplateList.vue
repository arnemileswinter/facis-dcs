<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import { useContractTemplateStateFilterStore } from '@/stores/contract-template-state-filter-store'
import { toComparableValue } from '@/utils/comparison'
import { storeToRefs } from 'pinia'
import { computed, onUnmounted, ref, type Ref } from 'vue'
import ListSort from '../ListSort.vue'
import ListSearch from './ListSearch.vue'
import TemplateListItem from './TemplateListItem.vue'
import TemplateListStateFilter from './TemplateListStateFilter.vue'

const props = defineProps<{
  items: PartialContractTemplate[]
  hasReviewTask: (template: PartialContractTemplate) => boolean
  hasApprovalTask: (template: PartialContractTemplate) => boolean
}>()

const sorter = new Map([
  ['created_at', 'Creation date'],
  ['updated_at', 'Update date'],
  ['state', 'Template state'],
  ['name', 'Name'],
])

const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const stateFilterStore = useContractTemplateStateFilterStore()
const { stateFilters } = storeToRefs(stateFilterStore)

const searchedItems: Ref<PartialContractTemplate[]> = ref(props.items)

const sortedItems = computed(() => {
  if (!sorter.has(sortBy.value)) {
    return searchedItems.value
  }
  return searchedItems.value.slice().sort((a, b) => {
    const aSortValue = a[sortBy.value as keyof PartialContractTemplate]
    const bSortValue = b[sortBy.value as keyof PartialContractTemplate]
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
  if (stateFilters.value.size > 0) {
    return sortedItems.value.filter((item) => stateFilters.value.has(item.state))
  }
  return sortedItems.value
})

function applySearchResult(searchResult: PartialContractTemplate[]) {
  searchedItems.value = searchResult
}

onUnmounted(() => stateFilterStore.reset())
</script>

<template>
  <ul class="list">
    <li class="tracking-wide px-4 flex justify-between flex-col sm:flex-row">
      <TemplateListStateFilter />
      <ListSearch :items="items" class="flex-1" @search-result="applySearchResult" />
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <TemplateListItem
      v-for="item in filteredItems"
      :key="`${item.did}|${item.document_number}|${item.version}`"
      :item="item"
      :has-review-task="props.hasReviewTask(item)"
      :has-approval-task="props.hasApprovalTask(item)"
    />
  </ul>
</template>
