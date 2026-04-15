<script setup lang="ts">
import NegotiationList from '@/components/lists/contract/negotiation/NegotiationList.vue'
import type { Contract } from '@/models/contract/contract'
import { ROUTES } from '@/router/router'
import { contractWorkflowService } from '@/services/contract-workflow-service'
import { useAuthStore } from '@/stores/auth-store'
import { ContractState } from '@/types/contract-state'
import { PencilSquareIcon } from '@heroicons/vue/20/solid'
import { computed, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

type ChangeRequest = Pick<Contract, 'name' | 'description' | 'contract_data'>

const route = useRoute()
const router = useRouter()

const authStore = useAuthStore()
const username = computed(() => authStore.user?.username)

const contract: Ref<Contract | null> = ref(null)
const name = ref<string | null>(null)
const description = ref<string | null>(null)

const toggle = ref({ name: false, description: false })
const hasChangeRequest = computed(() => {
  return (
    Object.values(toggle.value).some(Boolean) &&
    (name.value !== contract.value?.name || description.value !== contract.value.description)
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
        }
      } catch (err: any) {
        console.error('Failed to load contract', err)
      }
    }
  },
  { immediate: true },
)

watch(toggle, () => {
  if (toggle.value.name) {
    name.value = contract.value?.name ?? null
  }
  if (toggle.value.description) {
    description.value = contract.value?.description ?? null
  }
})

const toggleName = () => {
  toggle.value.name = name.value && name.value !== contract.value?.name ? toggle.value.name : !toggle.value.name
}

const toggleDesc = () => {
  toggle.value.description =
    description.value && description.value !== contract.value?.description
      ? toggle.value.description
      : !toggle.value.description
}

const negotiateContractChange = async () => {
  if (!contract.value || !username.value) return
  const changeRequest: ChangeRequest = {}
  if (name.value !== null && toggle.value.name) changeRequest.name = name.value
  if (description.value !== null && toggle.value.description) changeRequest.description = description.value
  console.log(changeRequest)
  const response = await contractWorkflowService.negotiate({
    did: contract.value?.did,
    updated_at: contract.value?.updated_at,
    negotiated_by: username.value,
    change_request: changeRequest,
  })
  if (response.did) {
    router.push({ name: ROUTES.CONTRACTS.LIST })
  }
}
</script>

<template>
  <div class="flex flex-col min-h-full -mx-4 md:-mx-8 -my-4 md:-my-8">
    <div v-if="!!contract" class="max-w-4xl mx-auto px-6 py-12">
      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Global Name</legend>
        <div class="flex">
          <input v-model="contract.name" class="input input-bordered w-full" type="text" name="name" disabled />
          <button class="btn btn-outline ml-3" @click="toggleName">
            <PencilSquareIcon class="size-4" />
          </button>
          <input v-show="toggle.name" type="text" v-model="name" class="input" name="name-request" />
        </div>
      </fieldset>

      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Base Description</legend>
        <div class="flex">
          <textarea
            v-model="contract.description"
            class="textarea textarea-bordered w-full h-24"
            name="description"
            disabled
          ></textarea>
          <button class="btn btn-outline ml-3" @click="toggleDesc">
            <PencilSquareIcon class="size-4" />
          </button>
          <textarea
            v-show="toggle.description"
            v-model="description"
            class="textarea"
            name="description-request"
          ></textarea>
        </div>
      </fieldset>

      <div class="mt-8" v-if="(contract.negotiations?.length ?? -1) > 0">
        <div>Active negotiations:</div>
        <NegotiationList :negotiations="contract.negotiations!" />
      </div>
    </div>
    <div class="sticky bottom-0 shrink-0 border-t border-base-300 bg-base-100">
      <div class="max-w-4xl mx-auto px-6 py-3 flex flex-col md:flex-row gap-3">
        <button class="btn btn-ghost md:w-32" @click="$router.back()">Cancel</button>
        <button
          v-if="contract?.state === ContractState.negotiation"
          class="btn btn-primary flex-1"
          :disabled="!hasChangeRequest"
          @click="negotiateContractChange"
        >
          Submit change request
        </button>
      </div>
    </div>
  </div>
</template>
