<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import type { ContractTemplateReviewTask } from '@/models/contract-template-review-task'
import { ROUTES } from '@/router/router'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplateReviewTaskStateFilterStore } from '@/stores/contract-template-review-task-state-filter-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { TemplateState } from '@/types/contract-template-state'
import { ContractTemplateReviewTaskState, reviewTaskStates } from '@/types/review-task-state'
import { toComparableValue } from '@/utils/comparison'
import { computed, onUnmounted, ref, type Ref } from 'vue'
import ListSort from '../../ListSort.vue'
import TemplateListSearch from '../TemplateListSearch.vue'
import TemplateListStateFilter from '../TemplateListStateFilter.vue'

const props = defineProps<{
  items: ContractTemplateReviewTask[]
}>()

const templatesStore = useContractTemplatesStore()
const authStore = useAuthStore()
const stateFilterStore = useContractTemplateReviewTaskStateFilterStore()

const sorter = new Map([
  ['created_at', 'Creation date'],
  ['state', 'Task state'],
])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const searchedItems: Ref<ContractTemplateReviewTask[]> = ref(props.items)

const sortedItems = computed(() => {
  if (!sorter.has(sortBy.value)) {
    return searchedItems.value
  }
  return searchedItems.value.slice().sort((taskA, taskB) => {
    const aSortValue = taskA[sortBy.value as keyof ContractTemplateReviewTask]
    const bSortValue = taskB[sortBy.value as keyof ContractTemplateReviewTask]
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

const templates = computed(() => {
  return templatesStore.contractTemplates.filter((template) =>
    props.items.map((task) => task.did).includes(template.did),
  )
})

const getTemplateName = (item: ContractTemplateReviewTask) => {
  return templatesStore.contractTemplates.find((template) => template.did === item.did)?.name ?? 'Nameless Template'
}

const canEdit = (item: ContractTemplateReviewTask) => {
  const template = templatesStore.contractTemplates.find((template) => template.did === item.did)
  const state = template?.state
  return (
    (template?.created_by === authStore.user?.username &&
      (state === TemplateState.draft || state === TemplateState.rejected)) ||
    state === TemplateState.submitted
  )
}

const resolveViewRouteName = (item: ContractTemplateReviewTask) => {
  if (item.state === ContractTemplateReviewTaskState.open) {
    return ROUTES.TEMPLATES.REVIEW
  }
  return ROUTES.TEMPLATES.VIEW
}

const applySearchResult = (searchResult: PartialContractTemplate[]) => {
  searchedItems.value = props.items.filter((task) => searchResult.map((template) => template.did).includes(task.did))
}

onUnmounted(() => stateFilterStore.reset())
</script>

<template>
  <ul class="list">
    <li class="tracking-wide w-full px-4 flex justify-end flex-col sm:flex-row">
      <TemplateListStateFilter label="Review Task" :filters="reviewTaskStates" store-type="reviewTasks" />
      <TemplateListSearch class="flex-1" :items="templates" @search-result="applySearchResult" />
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <li v-for="item in filteredItems" class="list-row">
      <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300">
        <div class="card-body">
          <h2 class="card-title flex-wrap justify-between">
            <div>Review Task for Contract Template: {{ getTemplateName(item) }}</div>
            <div class="badge badge-secondary">{{ item.state }}</div>
          </h2>
          <div class="flex justify-between">
            <div v-if="item.document_number">Document number: {{ item.document_number }}</div>
            <div v-if="item.version">Version: {{ item.version }}</div>
          </div>
          <div class="flex justify-between">
            <div>Creation date: {{ new Date(item.created_at).toLocaleDateString() }}</div>
            <div class="card-actions justify-end">
              <RouterLink
                :to="{
                  name: resolveViewRouteName(item),
                  params: { did: item.did },
                }"
                class="btn btn-sm btn-primary rounded-box"
              >
                View
              </RouterLink>
              <RouterLink
                v-if="canEdit(item)"
                :to="{
                  name: ROUTES.TEMPLATES.EDIT,
                  params: { did: item.did },
                }"
                class="btn btn-sm btn-secondary rounded-box gap-2"
              >
                Edit
              </RouterLink>
            </div>
          </div>
        </div>
      </div>
    </li>
  </ul>
</template>
