<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import type { ContractTemplateReviewTask } from '@/models/contract-template-review-task'
import type { ContractReviewTask } from '@/models/contract/contract-review-task'
import { ROUTES } from '@/router/router'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplateReviewTaskStateFilterStore } from '@/stores/contract-template-review-task-state-filter-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { useContractsStore } from '@/stores/contracts-store'
import { TemplateState } from '@/types/contract-template-state'
import { ReviewTaskState, reviewTaskStates } from '@/types/review-task-state'
import { toComparableValue } from '@/utils/comparison'
import { toProperCase } from '@/utils/string'
import { computed, onUnmounted, ref, type Ref } from 'vue'
import ListSort from '../../ListSort.vue'
import ListStateFilter from '../../ListStateFilter.vue'
import TemplateListSearch from '../../template/TemplateListSearch.vue'

const props = defineProps<{
  items: (ContractTemplateReviewTask | ContractReviewTask)[]
}>()

const templatesStore = useContractTemplatesStore()
const contractsStore = useContractsStore()
const authStore = useAuthStore()
const stateFilterStore = useContractTemplateReviewTaskStateFilterStore()

const sorter = new Map([
  ['created_at', 'Creation date'],
  ['state', 'Task state'],
])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const searchFilteredItems: Ref<(ContractTemplateReviewTask | ContractReviewTask)[]> = ref([])

const searchedItems = computed(() => {
  return searchFilteredItems.value.length > 0 ? searchFilteredItems.value : props.items
})

const sortedItems = computed(() => {
  if (!sorter.has(sortBy.value)) {
    return searchedItems.value
  }
  return searchedItems.value.slice().sort((taskA, taskB) => {
    const aSortValue = taskA[sortBy.value as keyof (ContractTemplateReviewTask | ContractReviewTask)]
    const bSortValue = taskB[sortBy.value as keyof (ContractTemplateReviewTask | ContractReviewTask)]
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

const getContractName = (item: ContractReviewTask) => {
  return contractsStore.contracts.find((contract) => contract.did === item.did)?.name ?? 'Nameless Contract'
}

const canEdit = (item: ContractTemplateReviewTask | ContractReviewTask) => {
  if (item.type === 'template') {
    const template = templatesStore.contractTemplates.find((template) => template.did === item.did)
    const state = template?.state
    return (
      (template?.created_by === authStore.user?.username &&
        (state === TemplateState.draft || state === TemplateState.rejected)) ||
      state === TemplateState.submitted
    )
  } else {
    // TODO:
    return false
  }
}

const resolveViewRouteName = (item: ContractTemplateReviewTask | ContractReviewTask) => {
  if (item.type === 'template') {
    if (item.state === ReviewTaskState.open) {
      return ROUTES.TEMPLATES.REVIEW
    }
    return ROUTES.TEMPLATES.VIEW
  } else {
    // TODO:
  }
}

const applySearchResult = (searchResult: PartialContractTemplate[]) => {
  searchFilteredItems.value = props.items.filter((task) =>
    searchResult.map((template) => template.did).includes(task.did),
  )
}

onUnmounted(() => stateFilterStore.reset())
</script>

<template>
  <ul class="list">
    <li class="tracking-wide w-full px-4 flex justify-end flex-col sm:flex-row">
      <ListStateFilter label="Review Task" :filters="reviewTaskStates" store-type="reviewTasks" />
      <TemplateListSearch class="flex-1" :items="templates" @search-result="applySearchResult" />
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <li v-for="item in filteredItems" class="list-row">
      <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300">
        <div class="card-body">
          <h2 class="card-title flex-wrap justify-between">
            <div v-if="item.type === 'template'">Review Task for Contract Template: {{ getTemplateName(item) }}</div>
            <div v-else>Review Task for Contract: {{ getContractName(item) }}</div>
            <div class="flex-1"></div>
            <div class="badge badge-accent">{{ toProperCase(item.type) }} Task</div>
            <div class="badge badge-secondary">{{ item.state }}</div>
          </h2>
          <div class="flex justify-between">
            <div v-if="item.type === 'template' && item.document_number">
              Document number: {{ item.document_number }}
            </div>
            <div v-if="item.type === 'template' && item.version">Version: {{ item.version }}</div>
            <div v-else-if="item.type === 'contract' && item.contract_version">Version: {{ item.contract_version }}</div>
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
