<script setup lang="ts">
import type { ContractTemplateApprovalTask } from '@/models/contract-template-approval-task'
import { ROUTES } from '@/router/router'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { TemplateState } from '@/types/contract-template-state'
import { toComparableValue } from '@/utils/comparison'
import { computed, ref } from 'vue'
import ListSort from '../ListSort.vue'

const props = defineProps<{
  items: ContractTemplateApprovalTask[]
}>()

const templatesStore = useContractTemplatesStore()

const sorter = new Map([['created_at', 'Creation date']])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const sortedItems = computed(() =>
  props.items.slice().sort((taskA, taskB) => {
    const aSortValue = taskA[sortBy.value as keyof ContractTemplateApprovalTask]
    const bSortValue = taskB[sortBy.value as keyof ContractTemplateApprovalTask]
    const aValue = toComparableValue(aSortValue)
    const bValue = toComparableValue(bSortValue)
    if (!aValue && !bValue) return 0
    if (!aValue) return sortOrder.value
    if (!bValue) return sortOrder.value * -1

    let result: number
    if (typeof aValue === 'number' && typeof bValue === 'number') {
      result = Math.sign(bValue - aValue)
    } else {
      result = String(bValue).localeCompare(String(aValue))
    }
    return sortOrder.value * result
  }),
)

const getTemplateName = (item: ContractTemplateApprovalTask) => {
  return templatesStore.contractTemplates.find((template) => template.did === item.did)?.name ?? 'Nameless Template'
}

const getTemplateState = (item: ContractTemplateApprovalTask) => {
  return templatesStore.contractTemplates.find((template) => template.did === item.did)?.state
}

const canApprove = (item: ContractTemplateApprovalTask) => {
  return item.state === 'OPEN' && getTemplateState(item) === TemplateState.reviewed
}

const resolveViewRouteName = (item: ContractTemplateApprovalTask) => {
  if (canApprove(item)) {
    return ROUTES.TEMPLATES.APPROVE
  }
  return ROUTES.TEMPLATES.VIEW
}
</script>

<template>
  <ul class="list">
    <li class="tracking-wide px-4 flex justify-end flex-col sm:flex-row">
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <li v-for="item in sortedItems" class="list-row">
      <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300">
        <div class="card-body">
          <h2 class="card-title flex-wrap justify-between">
            <div>Approval Task for Contract Template: {{ getTemplateName(item) }}</div>
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
            </div>
          </div>
        </div>
      </div>
    </li>
  </ul>
</template>
