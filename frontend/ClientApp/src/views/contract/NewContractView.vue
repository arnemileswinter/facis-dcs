<script setup lang="ts">
import SubmitSelectionDialog from '@/components/SubmitSelectionDialog.vue'
import type { PartialContractTemplate } from '@/models/contract-template'
import type { Contract } from '@/models/contract/contract'
import type { SelectedUserRole } from '@/models/user'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import type { SemanticConditionValueSetter } from '@/modules/contract-workflow-engine/models/contract-content-values-store'
import { useSemanticValueVerification } from '@/modules/contract-workflow-engine/composables/useSemanticValueVerification'
import type { VerificationResult } from '@/modules/contract-workflow-engine/composables/useSemanticValueVerification'
import { useErrorStore } from '@/stores/error-store'
import { ContractState } from '@/types/contract-state'
import ContractDetailsEditor from '@/modules/contract-workflow-engine/components/ContractDetailsEditor.vue'
import { useContractEditorUiStore } from '@/modules/contract-workflow-engine/store/contractEditorUiStore'
import { useContractContentValuesStore } from '@/modules/contract-workflow-engine/store/contractContentValuesStore'
import TemplatePreview from '@template-repository/components/builder-editor/preview/TemplatePreview.vue'
import { useTemplateDraftStore } from '@template-repository/store/templateDraftStore'
import { storeToRefs } from 'pinia'
import { computed, onUnmounted, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { ContractData } from '@/models/contract-data'

const route = useRoute()
const router = useRouter()

const errorStore = useErrorStore()
const templatesStore = useContractTemplatesStore()
const templateDraftStore = useTemplateDraftStore()
const contractContentValuesStore = useContractContentValuesStore()
const contractEditorUiStore = useContractEditorUiStore()
const { verifySemanticValue } = useSemanticValueVerification()
const { approvedTemplates } = storeToRefs(templatesStore)
const { activeTab, tabs } = storeToRefs(contractEditorUiStore)
const { setActiveTab } = contractEditorUiStore

const did = ref<string | null>(null)
const isEditMode = computed(() => !!route.params.did || !!did.value)
const isSubmitting = ref(false)
const selectedTemplate: Ref<PartialContractTemplate | null> = ref(null)
const verificationResult: Ref<VerificationResult | null> = ref(null)

const contract: Ref<Contract | null> = ref(null)

const canSubmit = computed(() => isEditMode.value || selectedTemplate.value !== null)
const setSemanticConditionValue = computed<SemanticConditionValueSetter>(() => {
  if (!isEditMode.value) return null
  return (blockId: string, conditionId: string, parameterName: string, parameterValue: string | number, subBlockId?: string) =>
    contractContentValuesStore.setSemanticConditionValue({ blockId, conditionId, parameterName, parameterValue, subBlockId })
})

const submit = async () => {
  isSubmitting.value = true
  try {
    if (!isEditMode.value && !!selectedTemplate.value) {
      const response = await contractWorkflowService.create({ did: selectedTemplate.value.did })
      did.value = response.did
      errorStore.add('Contract created.', 'info')
    } else if (contract.value) {
      const contractData: ContractData = {
        documentOutline: contract.value.contract_data?.documentOutline ?? [],
        documentBlocks: contract.value.contract_data?.documentBlocks ?? [],
        semanticConditions: contract.value.contract_data?.semanticConditions ?? [],
        subTemplateSnapshots: contract.value.contract_data?.subTemplateSnapshots ?? [],
        templateDataVersion: contract.value.contract_data?.templateDataVersion ?? 1,
        semanticConditionValues: contractContentValuesStore.semanticConditionValues,
      }
      await contractWorkflowService.update({
        did: contract.value.did,
        updated_at: contract.value.updated_at,
        name: contract.value.name,
        description: contract.value.description,
        contract_data: contractData,
      })
      router.push({ name: ROUTES.CONTRACTS.LIST })
    }
  } catch (error) {
    console.error('Submission failed', error)
  } finally {
    isSubmitting.value = false
  }
}

watch(
  isEditMode,
  async (value) => {
    if (value) {
      try {
        const id = did.value || route.params.did
        if (id && !Array.isArray(id)) {
          contract.value = await contractWorkflowService.retrieveById({ did: id })
          applyContractDataToDraft(contract.value?.contract_data)
        }
      } catch (err: any) {
        console.error('Failed to load contract')
      }
    } else {
      templatesStore.loadTemplates()
    }
  },
  { immediate: true },
)

onUnmounted(() => {
  templateDraftStore.reset()
  contractContentValuesStore.reset()
  contractEditorUiStore.reset()
  verificationResult.value = null
})

const submitContract = async (result: SelectedUserRole[]) => {
  if (!contract.value) return
  const isSemanticValueValid = verifySemanticValues()
  if (!isSemanticValueValid) return

  const reviewers = result.filter((user) => user.role === 'CONTRACT_REVIEWER').map((user) => user.user.username)
  const approver = result.find((user) => user.role === 'CONTRACT_APPROVER')?.user.username!
  const negotiators = result.filter((user) => user.role === 'CONTRACT_NEGOTIATOR').map((user) => user.user.username)
  const response = await contractWorkflowService.submit({
    did: contract.value?.did,
    updated_at: contract.value?.updated_at,
    reviewers,
    approver,
    negotiators,
  })
  if (response.did) {
    router.push({ name: ROUTES.CONTRACTS.LIST })
  }
}

const verifySemanticValues = (): boolean => {
  const combinedConditions = [...templateDraftStore.semanticConditions]
  templateDraftStore?.subTemplateSnapshots?.forEach((snapshot) => {
    combinedConditions.push(...(snapshot?.template_data?.semanticConditions ?? []))
  })
  const result = verifySemanticValue(combinedConditions, contractContentValuesStore.semanticConditionValues)
  verificationResult.value = result
  if (result.isValid) {
    return true
  }
  errorStore.add(`Semantic value verification failed: ${result.errors.length} issue(s).`, 'error')
  // go to content tab and highlight semantic inconsistencies
  setActiveTab('content')
  return false
}

// Contract data includes the template data used to fill the contract template
function applyContractDataToDraft(contractData?: unknown) {
  if (contractData == null) {
    templateDraftStore.reset()
    contractContentValuesStore.reset()
    verificationResult.value = null
    return
  }
  const cd = contractData as ContractData
  templateDraftStore.reset({
    documentOutline: cd.documentOutline ?? [],
    documentBlocks: cd.documentBlocks ?? [],
    semanticConditions: cd.semanticConditions ?? [],
    subTemplateSnapshots: cd.subTemplateSnapshots ?? [],
    templateDataVersion: cd.templateDataVersion ?? 1,
  })
  contractContentValuesStore.reset({ semanticConditionValues: cd.semanticConditionValues ?? [] })
  verificationResult.value = null
}
</script>

<template>
  <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">
    <div v-if="!isEditMode" class="max-w-4xl mx-auto px-6 py-12">
      <select v-model="selectedTemplate" class="select">
        <option :value="null" disabled selected>Pick a contract template</option>
        <option v-for="template in approvedTemplates" :key="template.did" :value="template">{{ template.name }}</option>
      </select>
    </div>
    <div v-else-if="!!contract">
      <div class="flex-1 flex flex-col">
        <!-- Tabs -->
        <div class="sticky top-0 z-10 shrink-0 bg-base-200 border-b border-base-300">
          <div class="max-w-4xl mx-auto px-6 pt-3">
            <p class="text-xs font-black uppercase tracking-widest text-base-content/40 mb-2">
              {{ isEditMode ? 'Update Contract' : 'Create Contract' }}
            </p>
            <div role="tablist" class="tabs tabs-lift tabs-lg">
              <a v-for="tab in tabs" :key="tab.id" role="tab" class="tab"
                :class="{ 'tab-active': activeTab === tab.id }" @click="setActiveTab(tab.id)">
                {{ tab.label }}
              </a>
            </div>
          </div>
        </div>
        <!-- Tab content -->
        <div class="grow mt-5">
          <div class="max-w-4xl mx-auto p-6">
            <div class="grid grid-cols-1 gap-4">
              <div v-show="activeTab === 'details'">
                <ContractDetailsEditor :contract="contract" />
              </div>
              <div v-show="activeTab === 'content'">
                <div class="card bg-base-100 border border-base-300 shadow-sm">
                  <div class="card-body gap-5">
                    <TemplatePreview :document-outline="templateDraftStore.documentOutline"
                      :document-blocks="templateDraftStore.documentBlocks"
                      :semantic-conditions="templateDraftStore.semanticConditions"
                      :semantic-condition-values="contractContentValuesStore.semanticConditionValues"
                      :verification-result="verificationResult"
                      :sub-template-snapshots="templateDraftStore.subTemplateSnapshots"
                      :set-semantic-condition-value="setSemanticConditionValue" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="$router.back()">Cancel</button>
        <button @click="submit" class="btn btn-primary flex-1" :disabled="isSubmitting || !canSubmit">
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          {{ isEditMode ? 'Update Template' : 'Create' }}
        </button>
        <SubmitSelectionDialog v-if="isEditMode && contract?.state === ContractState.draft" dialog-type="contract"
          @submit="submitContract" class="btn btn-primary flex-1" />
      </div>
    </div>
  </div>
</template>
