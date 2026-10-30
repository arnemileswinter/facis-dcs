<script setup lang="ts">
import { type Ref, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useContractEditorUiStore } from '@contract-workflow-engine/store/contractEditorUiStore'
import ContractAuditList from '@/components/lists/contract/ContractAuditList.vue'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import type { ContractAuditResponse } from '@/models/responses/contract-response'

const route = useRoute()
const editorStore = useContractEditorUiStore()
const data: Ref<ContractAuditResponse> = ref([])

const isLoading = ref(false)
const loadError = ref('')
const justification = ref('')

const loadAudit = async () => {
  const did = route.params.did
  if (!did || Array.isArray(did)) return
  loadError.value = ''
  try {
    isLoading.value = true
    data.value = await contractWorkflowService.audit({ did, justification: justification.value.trim() })
  } catch (err) {
    data.value = []
    loadError.value = err instanceof Error ? err.message : 'The audit trail could not be loaded'
  } finally {
    isLoading.value = false
  }
}

watch(
  () => editorStore.activeTab === 'audit',
  (value) => {
    if (!value) {
      data.value = []
      loadError.value = ''
    }
  },
)
</script>

<template>
  <form class="mb-3 flex items-end gap-2" @submit.prevent="loadAudit">
    <label class="form-control grow">
      <span class="label-text mb-1">Audit justification</span>
      <input v-model="justification" class="input-bordered input input-sm" required />
    </label>
    <button type="submit" class="btn btn-sm btn-primary" :disabled="isLoading || !justification.trim()">
      Load audit trail
    </button>
  </form>
  <div v-if="isLoading" class="loading loading-sm loading-spinner"></div>
  <div v-else-if="loadError" role="alert" class="alert alert-error">{{ loadError }}</div>
  <div v-else-if="data.length < 1">No audit data</div>
  <ContractAuditList v-else :audits="data" />
</template>
