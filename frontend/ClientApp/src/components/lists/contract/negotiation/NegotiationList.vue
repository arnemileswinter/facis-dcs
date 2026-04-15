<script setup lang="ts">
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import type { ContractNegotiation } from '@/models/contract/contract-negotiation';
import { contractWorkflowService } from '@/services/contract-workflow-service';
import { useAuthStore } from '@/stores/auth-store';
import { computed, useTemplateRef } from 'vue';

defineProps<{
  negotiations: ContractNegotiation[]
}>()

const authStore = useAuthStore()
const username = computed(() => authStore.user?.username)

const confirmationModal = useTemplateRef<InstanceType<typeof ConfirmationModal>>('confirmation-modal')

const acceptNegotiation = async (negotiation: ContractNegotiation) => {
  if (!username.value || !confirmationModal.value) return
  try {
    const { isCanceled } = await confirmationModal.value?.reveal({ message: 'Accept this change request?' })
    if (!isCanceled) {
      await contractWorkflowService.respond({
        id: negotiation.id,
        action_flag: 'ACCEPTING',
        responded_by: username.value,
      })
    }
  } catch (err) {
    console.error('Accepting the negotiation failed', err)
  }
}

const rejectNegotiation = async (negotiation: ContractNegotiation) => {
  if (!username.value || !confirmationModal.value) return
  try {
    const rejectResult = await confirmationModal.value.reveal({
      message: 'Reject this change request?',
      editor: { requiredText: true, placeholder: 'Rejection reason' },
    })
    if (!rejectResult.isCanceled) {
      await contractWorkflowService.respond({
        id: negotiation.id,
        action_flag: 'REJECTING',
        responded_by: username.value,
        RejectionReason: rejectResult.data,
      })
    }
  } catch (err) {
    console.error('Rejecting the negotiation failed', err)
  }
}
</script>

<template>
  <ul class="list">
    <li v-for="negotiation in negotiations" :key="negotiation.id" class="list-row">
      <div class="card bg-base-200 card-border">
        <div class="card-body">
          <h2 class="card-title">Change request proposed by: {{ negotiation.created_by }}</h2>
          <div class="m-2 bg-base-100 rounded-box p-2">
            <pre>{{ JSON.stringify(negotiation.change_request, null, 2) }}</pre>
          </div>
          <ul class="list">
            <li>Decisions</li>
            <li
              v-for="decision in negotiation.negotiation_decisions"
              :key="decision.negotiator"
              class="list-row flex justify-between"
            >
              <div>{{ decision.negotiator }}</div>
              <div class="badge badge-sm badge-accent">{{ decision.decision ?? 'PENDING' }}</div>
            </li>
          </ul>
          <div class="card-actions justify-end">
            <button class="btn btn-sm btn-primary" @click="acceptNegotiation(negotiation)">Accept</button>
            <button class="btn btn-sm btn-secondary" @click="rejectNegotiation(negotiation)">Reject</button>
          </div>
        </div>
      </div>
    </li>
  </ul>
  <ConfirmationModal ref="confirmation-modal" />
</template>
