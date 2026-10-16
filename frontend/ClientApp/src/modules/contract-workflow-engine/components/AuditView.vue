<script setup lang="ts">
import { type Ref, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useContractEditorUiStore } from '@contract-workflow-engine/store/contractEditorUiStore'
import ContractAuditList from '@/components/lists/contract/ContractAuditList.vue'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import ContractEvidencePanel from './ContractEvidencePanel.vue'
import type { Contract } from '@/models/contract/contract'
import type { ContractAuditResponse } from '@/models/responses/contract-response'

const props = defineProps<{ contract: Contract }>()
const route = useRoute()
const editorStore = useContractEditorUiStore()
const data: Ref<ContractAuditResponse> = ref([])

const isLoading = ref(false)
const loadError = ref('')

watch(
  [() => editorStore.activeTab, () => route.params.did],
  async ([tab, did], _, onCleanup) => {
    let current = true
    onCleanup(() => {
      current = false
    })
    data.value = []
    loadError.value = ''
    isLoading.value = false
    if (tab !== 'audit' || typeof did !== 'string') return
    isLoading.value = true
    try {
      const result = await contractWorkflowService.audit({ did })
      if (current) data.value = result
    } catch (err) {
      if (current) loadError.value = err instanceof Error ? err.message : 'The audit trail could not be loaded'
    } finally {
      if (current) isLoading.value = false
    }
  },
  { immediate: true },
)
</script>

<template>
  <ContractEvidencePanel
    v-if="editorStore.activeTab === 'audit' && route.params.did === props.contract.did"
    :key="props.contract.did"
    :contract="props.contract"
  />
  <div v-if="isLoading" class="loading loading-sm loading-spinner"></div>
  <div v-else-if="loadError" role="alert" class="alert alert-error">{{ loadError }}</div>
  <div v-else-if="data.length < 1">No audit data</div>
  <ContractAuditList v-else :audits="data" />
</template>
