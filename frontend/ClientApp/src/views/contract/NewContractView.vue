<script setup lang="ts">
import SubmitSelectionDialog from '@/components/SubmitSelectionDialog.vue'
import type { PartialContractTemplate } from '@/models/contract-template'
import type { Contract } from '@/models/contract/contract'
import type { SelectedUserRole } from '@/models/user'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { useErrorStore } from '@/stores/error-store'
import { ContractState } from '@/types/contract-state'
import { storeToRefs } from 'pinia'
import { computed, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const errorStore = useErrorStore()
const templatesStore = useContractTemplatesStore()
const { approvedTemplates } = storeToRefs(templatesStore)

const did = ref<string | null>(null)
const isEditMode = computed(() => !!route.params.did || !!did.value)
const isSubmitting = ref(false)
const selectedTemplate: Ref<PartialContractTemplate | null> = ref(null)

const contract: Ref<Contract | null> = ref(null)

const canSubmit = computed(() => isEditMode.value || selectedTemplate.value !== null)

const submit = async () => {
  isSubmitting.value = true
  try {
    if (!isEditMode.value && !!selectedTemplate.value) {
      const response = await contractWorkflowService.create({ did: selectedTemplate.value.did })
      did.value = response.did
      errorStore.add('Contract created.', 'info')
    } else if (contract.value) {
      await contractWorkflowService.update({
        did: contract.value.did,
        updated_at: contract.value.updated_at,
        name: contract.value.name,
        description: contract.value.description,
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
        }
      } catch (err: any) {
        console.error('Failed to load contract')
      }
    }
  },
  { immediate: true },
)

const submitContract = async (result: SelectedUserRole[]) => {
  if (!contract.value) return
  const reviewers = result.filter((user) => user.role === 'CONTRACT_REVIEWER').map((user) => user.user.username)
  const approver = result.find((user) => user.role === 'CONTRACT_APPROVER')?.user.username!
  const response = await contractWorkflowService.submit({
    did: contract.value?.did,
    updated_at: contract.value?.updated_at,
    reviewers,
    approver,
  })
  if (response.did) {
    router.push({ name: ROUTES.CONTRACTS.LIST })
  }
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
    <div v-else-if="!!contract" class="max-w-4xl mx-auto px-6 py-12">
      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Global Name</legend>
        <input v-model="contract.name" class="input input-bordered w-full" type="text" required />
      </fieldset>

      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Base Description</legend>
        <textarea v-model="contract.description" class="textarea textarea-bordered w-full h-24" required></textarea>
      </fieldset>
    </div>
    <div class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="$router.back()">Cancel</button>
        <button @click="submit" class="btn btn-primary flex-1" :disabled="isSubmitting || !canSubmit">
          <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
          {{ isEditMode ? 'Update Template' : 'Create' }}
        </button>
        <SubmitSelectionDialog
          v-if="isEditMode && contract?.state === ContractState.draft"
          dialog-type="contract"
          @submit="submitContract"
          class="btn btn-primary flex-1"
        />
      </div>
    </div>
  </div>
</template>
