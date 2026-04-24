<script setup lang="ts">
import NegotiationList from '@/components/lists/contract/negotiation/NegotiationList.vue'
import type { Contract, ContractChangeRequest } from '@/models/contract/contract'
import ContractDetailsEditor from '@/modules/contract-workflow-engine/components/ContractDetailsEditor.vue'
import { useContractContentValuesStore } from '@/modules/contract-workflow-engine/store/contractContentValuesStore'
import { useContractEditorUiStore } from '@/modules/contract-workflow-engine/store/contractEditorUiStore'
import TemplatePreview from '@/modules/template-repository/components/builder-editor/preview/TemplatePreview.vue'
import { useTemplateDraftStore } from '@/modules/template-repository/store/templateDraftStore'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'
import { ContractState } from '@/types/contract-state'
import { storeToRefs } from 'pinia'
import { computed, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const authStore = useAuthStore()
const templateDraftStore = useTemplateDraftStore()
const contractEditorUiStore = useContractEditorUiStore()
const { activeTab } = storeToRefs(contractEditorUiStore)
const { setActiveTab } = contractEditorUiStore
const contractContentValuesStore = useContractContentValuesStore()

const username = computed(() => authStore.user?.username)
const isSubmitting = ref(false)

const tabs = computed(() => contractEditorUiStore.availableTabs(contract.value?.state ?? ContractState.draft))

const contract: Ref<Contract | null> = ref(null)
const contractData: Ref<Contract | null> = ref(null)

const hasChangeRequest = computed(() => {
  return (
    contractData.value?.name !== contract.value?.name || contractData.value?.description !== contract.value?.description
  )
})

watch(
  () => !!route.params.did,
  async (value) => {
    if (value) {
      try {
        const id = route.params.did
        if (id && !Array.isArray(id)) {
          contract.value = await contractWorkflowService.retrieveById({ did: id })
          contractData.value = !!contract.value ? { ...contract.value } : null
        }
      } catch (err: any) {
        console.error('Failed to load contract', err)
      }
    }
  },
  { immediate: true },
)

const negotiateContractChange = async () => {
  if (!contract.value || !contractData.value || !username.value) return
  isSubmitting.value = true
  try {
    const changeRequest: ContractChangeRequest = {}
    if (contractData.value.name !== contract.value.name) {
      changeRequest.name = contractData.value.name
    }
    if (contractData.value.description !== contract.value.description) {
      changeRequest.description = contractData.value.description
    }
    console.log(changeRequest)
    const response = await contractWorkflowService.negotiate({
      did: contract.value?.did,
      updated_at: contract.value?.updated_at,
      negotiated_by: username.value,
      change_request: changeRequest,
    })
    if (response.did) {
      router.push({ name: ROUTES.TASKS.NEGOTIATIONS })
    }
  } catch (err) {
    console.error('Failed to submit change request', err)
  } finally {
    isSubmitting.value = false
  }
}

const submitContract = async () => {
  if (!contract.value) return
  try {
    const response = await contractWorkflowService.submit({
      did: contract.value.did,
      updated_at: contract.value.updated_at,
    })
    if (response.did) {
      router.push({ name: ROUTES.TASKS.NEGOTIATIONS })
    }
  } catch (err) {
    console.error('Failed to submit', err)
  }
}

const hasOpenDecisions = computed(() =>
  contract.value?.negotiations?.every((negotiation) =>
    negotiation.negotiation_decisions.every((decision) => !!decision.decision),
  ),
)
</script>

<template>
  <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">
    <div v-if="!!contract && !!contractData">
      <div class="flex-1 flex flex-col">
        <!-- Tabs -->
        <div class="sticky top-0 z-10 shrink-0 bg-base-200 border-b border-base-300">
          <div class="max-w-4xl mx-auto px-6 pt-3">
            <p class="text-xs font-black uppercase tracking-widest text-base-content/40 mb-2">Negotiate Contract</p>
            <div role="tablist" class="tabs tabs-lift tabs-lg">
              <a
                v-for="tab in tabs"
                :key="tab.id"
                role="tab"
                class="tab"
                :class="{ 'tab-active': activeTab === tab.id }"
                @click="setActiveTab(tab.id)"
              >
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
                <ContractDetailsEditor :contract="contractData" />
              </div>

              <div v-show="activeTab === 'content'">
                <div class="card bg-base-100 border border-base-300 shadow-sm">
                  <div class="card-body gap-5">
                    <div>
                      <TemplatePreview
                        :document-outline="templateDraftStore.documentOutline"
                        :document-blocks="templateDraftStore.documentBlocks"
                        :semantic-conditions="templateDraftStore.semanticConditions"
                        :semantic-condition-values="contractContentValuesStore.semanticConditionValues"
                        :sub-template-snapshots="templateDraftStore.subTemplateSnapshots"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="max-w-4xl mx-auto mt-8 p-6" v-if="(contract.negotiations?.length ?? -1) > 0">
        <div>Active negotiations:</div>
        <NegotiationList :contract="contract" />
      </div>
    </div>
    <div class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="$router.back()">Cancel</button>
        <button
          v-if="contract?.state === ContractState.negotiation"
          class="btn btn-primary flex-1"
          :disabled="isSubmitting || !hasChangeRequest"
          @click="negotiateContractChange"
        >
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          Submit change request
        </button>
        <button
          v-if="contract?.state === ContractState.negotiation"
          class="btn btn-primary flex-1"
          :disabled="isSubmitting || !hasOpenDecisions"
          @click="submitContract"
        >
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          Submit contract
        </button>
      </div>
    </div>
  </div>
</template>
