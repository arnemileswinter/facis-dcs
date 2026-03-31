<script setup lang="ts">
import ContractList from '@/components/lists/contract/ContractList.vue'
import type { Contract } from '@/models/contract/contract'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { storeToRefs } from 'pinia'
import { computed, onMounted, ref, type Ref } from 'vue'

const loading = ref(true)
const error = ref<string | null>(null)

const contracts: Ref<Contract[]> = ref([])

const authStore = useAuthStore()
const templatesStore = useContractTemplatesStore()
const { hasApprovedTemplates } = storeToRefs(templatesStore)

async function loadContracts() {
  loading.value = true
  error.value = null
  try {
    const result = await contractWorkflowService.retrieve()
    contracts.value = result.contracts
  } catch (err: any) {
    error.value = err.message || 'Error loading contracts'
  } finally {
    loading.value = false
  }
}
console.log(authStore.user?.roles)
const isContractCreator = computed(() => authStore.user?.roles?.some((role) => ['CONTRACT_CREATOR'].includes(role)))

onMounted(loadContracts)
</script>

<template>
  <div class="flex justify-between p-4 mb-4">
    <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight">
      {{ $route.meta.name }}
    </h2>

    <RouterLink
      v-if="hasApprovedTemplates && isContractCreator"
      :to="{ name: ROUTES.CONTRACTS.NEW }"
      class="btn rounded-box self-end btn-secondary gap-2"
      #default="{ route }"
    >
      {{ route.meta.name }}
    </RouterLink>
    <div v-else></div>
  </div>
  <div>
    <div v-if="loading">Loading Contracts...</div>
    <div v-else-if="error">{{ error }}</div>
    <div v-else>
      <ContractList :items="contracts" />
    </div>
  </div>
</template>
