<script setup lang="ts">
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import type { Contract } from '@/models/contract/contract'
import type { ContractNegotiation } from '@/models/contract/contract-negotiation'
import type { ContractNegotiationDecision } from '@/models/contract/contract-negotiation-decision'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'
import { computed, ref, useTemplateRef } from 'vue'

const props = defineProps<{
  contract: Contract
}>()

const authStore = useAuthStore()
const username = computed(() => authStore.user?.username)

const confirmationModal = useTemplateRef<InstanceType<typeof ConfirmationModal>>('confirmation-modal')

const negotiations = computed(() => props.contract.negotiations ?? [])

const sortedNegotiations = computed(() =>
  negotiations.value.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()),
)

const sortedDecisions = (decisions: ContractNegotiationDecision[]) => {
  return decisions.sort((a, b) => a.negotiator.localeCompare(b.negotiator))
}

const isSubmitting = ref(false)

const acceptNegotiation = async (negotiation: ContractNegotiation) => {
  if (!username.value || !confirmationModal.value) return
  isSubmitting.value = true
  try {
    const { isCanceled } = await confirmationModal.value?.reveal({ message: 'Accept this change request?' })
    if (!isCanceled) {
      await contractWorkflowService.respond({
        id: negotiation.id,
        did: props.contract.did,
        action_flag: 'ACCEPTING',
        responded_by: username.value,
      })
      const decision = negotiation.negotiation_decisions.find((decision) => decision.negotiator === username.value)
      if (decision) decision.decision = 'ACCEPTED'
    }
  } catch (err) {
    console.error('Accepting the negotiation failed', err)
  } finally {
    isSubmitting.value = false
  }
}

const rejectNegotiation = async (negotiation: ContractNegotiation) => {
  if (!username.value || !confirmationModal.value) return
  isSubmitting.value = true
  try {
    const rejectResult = await confirmationModal.value.reveal({
      message: 'Reject this change request?',
      editor: { requiredText: true, placeholder: 'Rejection reason' },
    })
    if (!rejectResult.isCanceled) {
      await contractWorkflowService.respond({
        id: negotiation.id,
        did: props.contract.did,
        action_flag: 'REJECTING',
        responded_by: username.value,
        rejection_reason: rejectResult.data,
      })
      negotiation.negotiation_decisions.forEach((decision) => {
        if (decision.negotiator === username.value) {
          decision.decision = 'REJECTED'
          decision.rejection_reason = rejectResult.data
        } else {
          decision.decision = 'CLOSED'
        }
      })
    }
  } catch (err) {
    console.error('Rejecting the negotiation failed', err)
  } finally {
    isSubmitting.value = false
  }
}

const isBtnDisabled = (negotiation: ContractNegotiation) => {
  const decision = negotiation.negotiation_decisions.find((decision) => decision.negotiator === username.value)
  return decision?.decision !== undefined
}
</script>

<template>
  <ul class="list">
    <li v-for="negotiation in sortedNegotiations" :key="negotiation.id" class="list-row">
      <div class="card bg-base-200 shadow-md card-border">
        <div class="card-body">
          <h2 class="card-title">Change request proposed by: {{ negotiation.created_by }}</h2>
          <div class="m-2 card bg-base-100 shadow-md p-2">
            <pre class="whitespace-pre-wrap">{{ JSON.stringify(negotiation.change_request, null, 2) }}</pre>
          </div>
          <ul class="list">
            <li class="text-lg">Decisions</li>
            <li
              v-for="decision in sortedDecisions(negotiation.negotiation_decisions)"
              :key="decision.negotiator"
              class="list-row"
            >
              <div class="list-col-grow flex w-full justify-between">
                <div>{{ decision.negotiator }}</div>
                <div class="badge badge-sm badge-accent">{{ decision.decision ?? 'PENDING' }}</div>
              </div>
              <div v-if="decision.decision === 'REJECTED' && decision.rejection_reason" class="list-col-wrap truncate">
                Reason: {{ decision.rejection_reason }}
              </div>
            </li>
          </ul>
          <div class="card-actions justify-end">
            <button
              class="btn btn-sm btn-primary"
              :disabled="isSubmitting || isBtnDisabled(negotiation)"
              @click="acceptNegotiation(negotiation)"
            >
              <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
              Accept
            </button>
            <button
              class="btn btn-sm btn-secondary"
              :disabled="isSubmitting || isBtnDisabled(negotiation)"
              @click="rejectNegotiation(negotiation)"
            >
              <span v-if="isSubmitting" class="loading loading-spinner loading-sm"></span>
              Reject
            </button>
          </div>
        </div>
      </div>
    </li>
  </ul>
  <ConfirmationModal ref="confirmation-modal" />
</template>
