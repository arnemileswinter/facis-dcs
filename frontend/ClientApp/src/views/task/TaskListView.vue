<script setup lang="ts">
import ApprovalTaskList from '@/components/lists/task/approval/ApprovalTaskList.vue'
import NegotiationTaskList from '@/components/lists/task/negotiation/NegotiationTaskList.vue'
import ReviewTaskList from '@/components/lists/task/review/ReviewTaskList.vue'
import { ROUTES } from '@/router/router'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { useContractsStore } from '@/stores/contracts-store'
import { useErrorStore } from '@/stores/error-store'
import type { UserRole } from '@/types/user-role'
import { computed, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const authStore = useAuthStore()
const errorStore = useErrorStore()

const templatesStore = useContractTemplatesStore()
const contractsStore = useContractsStore()
const reviewTasks = computed(() => [...templatesStore.reviewTasks, ...contractsStore.reviewTasks])
const approvalTasks = computed(() => [...templatesStore.approvalTasks, ...contractsStore.approvalTasks])
const negotiationTasks = computed(() => contractsStore.negotiationTasks)

const hasTemplateRole = computed(() => {
  return (
    authStore.user?.roles?.some((role) =>
      (['TEMPLATE_CREATOR', 'TEMPLATE_REVIEWER', 'TEMPLATE_APPROVER', 'TEMPLATE_MANAGER'] as UserRole[]).includes(role),
    ) ?? false
  )
})

const hasContractRole = computed(() => {
  return (
    authStore.user?.roles?.some((role) =>
      (['CONTRACT_CREATOR', 'CONTRACT_REVIEWER', 'CONTRACT_APPROVER', 'CONTRACT_MANAGER'] as UserRole[]).includes(role),
    ) ?? false
  )
})

const loadTasks = async () => {
  if (route.name !== ROUTES.TASKS.NEGOTIATIONS && !templatesStore.hasTemplates && hasTemplateRole.value) {
    await templatesStore.loadTemplates()
  }
  if (!contractsStore.hasContracts && hasContractRole.value) {
    await contractsStore.loadContracts()
  }
}

const redirectOnEmptyTasks = async () => {
  await loadTasks()
  await nextTick()
  if (route.name === ROUTES.TASKS.REVIEWS && reviewTasks.value.length < 1) {
    errorStore.add('No review tasks assigned', 'info')
    router.back()
  } else if (route.name === ROUTES.TASKS.APPROVALS && approvalTasks.value.length < 1) {
    errorStore.add('No approval tasks assigned', 'info')
    router.back()
  }
}

watch(
  () => route.name,
  () => redirectOnEmptyTasks(),
  { immediate: true },
)
</script>

<template>
  <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight p-4 mb-4">
    {{ $route.meta.name }}
  </h2>

  <template v-if="$route.name === ROUTES.TASKS.REVIEWS">
    <ReviewTaskList :items="reviewTasks" />
  </template>
  <template v-else-if="$route.name === ROUTES.TASKS.APPROVALS">
    <ApprovalTaskList :items="approvalTasks" />
  </template>
  <template v-else-if="$route.name === ROUTES.TASKS.NEGOTIATIONS">
    <NegotiationTaskList :items="negotiationTasks" />
  </template>
</template>
