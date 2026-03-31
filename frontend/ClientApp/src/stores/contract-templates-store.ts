import type { PartialContractTemplate } from '@/models/contract-template'
import type { ContractTemplateApprovalTask } from '@/models/contract-template-approval-task'
import type { ContractTemplateReviewTask } from '@/models/contract-template-review-task'
import { TemplateState } from '@/types/contract-template-state'
import { defineStore } from 'pinia'
import { computed, ref, type Ref } from 'vue'

export const useContractTemplatesStore = defineStore('contractTemplates', () => {
  const contractTemplates: Ref<PartialContractTemplate[]> = ref([])
  const reviewTasks: Ref<ContractTemplateReviewTask[]> = ref([])
  const approvalTasks: Ref<ContractTemplateApprovalTask[]> = ref([])

  const hasTemplates = computed(() => contractTemplates.value.length > 0)
  const hasApprovedTemplates = computed(() => contractTemplates.value.some(template => template.state === TemplateState.approved))
  const approvedTemplates = computed(() => contractTemplates.value.filter(template => template.state === TemplateState.approved))

  return { contractTemplates, reviewTasks, approvalTasks, hasTemplates, hasApprovedTemplates, approvedTemplates }
})
