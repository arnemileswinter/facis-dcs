<script setup lang="ts">
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import type { PartialContractTemplate } from '@/models/contract-template'
import { contractTemplateService } from '@/services/contract-template-service'
import { useAuthStore } from '@/stores/auth-store'
import { TemplateState, type ContractTemplateState } from '@/types/contract-template-state'
import { computed, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

defineOptions({
  inheritAttrs: false,
})

const props = defineProps<{
  item: PartialContractTemplate
}>()

const confirmationModal = useTemplateRef('confirmation-modal')

const router = useRouter()
const authStore = useAuthStore()

const isManager = computed(() => {
  return authStore.user?.roles?.includes('TEMPLATE_MANAGER') ?? false
})

const canArchive = computed(() => {
  const archiveStates: ContractTemplateState[] = [TemplateState.deleted, TemplateState.deprecated]
  return isManager.value && !archiveStates.includes(props.item.state)
})

const canRegister = computed(() => {
  return isManager.value && props.item.state === TemplateState.approved
})

const archive = async () => {
  try {
    const { isCanceled } = await confirmationModal.value!.reveal({ message: 'Proceed with archiving?' })
    if (!isCanceled) {
      await contractTemplateService.archive({ did: props.item.did, updated_at: props.item.updated_at })
      router.go(0)
    }
  } catch (err) {
    console.error('Archiving failed:', err)
  }
}
const register = async () => {
  try {
    const { isCanceled } = await confirmationModal.value!.reveal({ message: 'Proceed with registration?' })
    if (!isCanceled) {
      await contractTemplateService.register({ did: props.item.did, updated_at: props.item.updated_at })
      router.go(0)
    }
  } catch (err) {
    console.error('Registration failed:', err)
  }
}

const audit = async () => {
  try {
    await contractTemplateService.audit({ did: props.item.did })
  } catch (err) {
    console.error('Audit failed:', err)
  }
}
</script>

<template>
  <button v-if="canRegister" @click="register" class="btn btn-sm btn-primary rounded-box mb-1">Register</button>
  <button v-if="canArchive" @click="archive" class="btn btn-sm btn-primary hover:btn-error rounded-box mb-1">Archive</button>
  <button v-if="isManager" @click="audit" class="btn btn-sm btn-secondary rounded-box">Audit</button>
  <ConfirmationModal ref="confirmation-modal" />
</template>
