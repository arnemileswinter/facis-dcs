<script setup lang="ts">
import type { ContractTemplateReviewTask } from '@/models/contract-template-review-task'
import { ROUTES } from '@/router/router'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { TemplateState } from '@/types/contract-template-state'
import { toComparableValue } from '@/utils/comparison'
import { computed, ref } from 'vue'
import ListSort from '../ListSort.vue'

const props = defineProps<{
  items: ContractTemplateReviewTask[]
}>()

const templatesStore = useContractTemplatesStore()
const authStore = useAuthStore()

const sorter = new Map([['created_at', 'Creation date']])
const defaultSort = sorter.keys().next().value!
const sortBy = ref(defaultSort)
const sortOrder = ref(1)

const sortedItems = computed(() =>
  props.items.slice().sort((taskA, taskB) => {
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
      result = String(bValue).localeCompare(String(aValue))
    }
    return sortOrder.value * result
  }),
)

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
  if (item.state === 'OPEN') {
    return ROUTES.TEMPLATES.REVIEW
  }
  return ROUTES.TEMPLATES.VIEW
}
</script>

<template>
  <ul class="list">
    <li class="tracking-wide w-full px-4 flex justify-end flex-col sm:flex-row">
      <ListSort :sorter="sorter" v-model:sort-by="sortBy" v-model:sort-order="sortOrder" />
    </li>
    <li v-for="item in sortedItems" class="list-row">
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
