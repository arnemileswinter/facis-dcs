<script setup lang="ts">
import type { PartialContractTemplate } from '../../../models/contract-template'

const props = defineProps<{
  item: PartialContractTemplate
}>()
</script>

<template>
  <li class="list-row">
    <div class="list-col-grow card bg-base-200 card-border hover:bg-base-300">
      <div class="card-body">
        <h2 class="card-title justify-between">
          <div>Name: {{ item.name }}</div>
          <div class="badge badge-secondary">{{ item.state }}</div>
        </h2>
        <div class="flex justify-between">
          <div>Document number: {{ item.document_number }}</div>
          <div>Version: {{ item.version }}</div>
        </div>
        <div class="flex justify-between">
          <div>Creation date: {{ new Date(item.created_at).toLocaleDateString() }}</div>
        <div class="card-actions justify-end">
          <button class="btn btn-sm rounded-box btn-primary">View</button>
          <RouterLink
            :to="{
              name: 'templates.edit',
              params: { did: item.did },
              query: { document_number: item.document_number, version: item.version },
            }"
            class="btn btn-sm rounded-box btn-secondary gap-2"
          >
            Edit
          </RouterLink>
        </div>
        </div>
      </div>
    </div>
  </li>
</template>
