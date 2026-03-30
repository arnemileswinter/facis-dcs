<script setup lang="ts">
import ApprovalTaskList from '@/components/lists/template/approval-task-list/ApprovalTaskList.vue'
import ReviewTaskList from '@/components/lists/template/review-task-list/ReviewTaskList.vue'
import { ROUTES } from '@/router/router'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { storeToRefs } from 'pinia'

const templatesStore = useContractTemplatesStore()
const { reviewTasks, approvalTasks } = storeToRefs(templatesStore)
</script>

<template>
  <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight p-4 mb-4">
    {{ $route.meta.name }}
  </h2>

  <div v-if="$route.name === ROUTES.TEMPLATES.TASKS.REVIEW">
    <ReviewTaskList :items="reviewTasks" />
  </div>
  <div v-else-if="$route.name === ROUTES.TEMPLATES.TASKS.APPROVAL">
    <ApprovalTaskList :items="approvalTasks" />
  </div>
</template>
