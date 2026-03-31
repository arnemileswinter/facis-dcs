<script setup lang="ts">
import type { Contract } from '@/models/contract/contract'
import { ROUTES } from '@/router/router'

defineProps<{
  item: Contract
}>()
</script>

<template>
  <li class="list-row min-w-0 w-full">
    <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300 min-w-0 w-full">
      <div class="card-body min-w-0">
        <h2 class="card-title flex-wrap sm:justify-between">
          <div class="flex gap-8 sm:h-full">
            <div>Name: {{ item.name }}</div>
            <!-- <div class="badge sm:badge-md badge-accent sm:h-full">{{ toProperCase(item.template_type) }}</div> -->
          </div>
          <div class="badge badge-secondary">{{ item.state }}</div>
        </h2>
        <div class="flex justify-between">
          <div v-if="item.contract_version">Version: {{ item.contract_version }}</div>
        </div>
        <div class="flex justify-between min-w-0">
          <div>Creation date: {{ new Date(item.created_at).toLocaleDateString() }}</div>
          <div v-if="item.description" class="px-10 flex-1 min-w-0 truncate hidden sm:block">
            {{ item.description }}
          </div>
          <div class="card-actions justify-end">
            <RouterLink to="#" class="btn btn-sm btn-primary rounded-box btn-disabled"> View </RouterLink>
            <RouterLink
              :to="{
                name: ROUTES.CONTRACTS.EDIT,
                params: { did: item.did },
              }"
              class="btn btn-sm btn-primary rounded-box gap-2"
            >
              Edit
            </RouterLink>
          </div>
        </div>
      </div>
    </div>
  </li>
</template>
