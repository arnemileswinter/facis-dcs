<template>
  <div class="card bg-base-100 border border-base-300 shadow-sm">
    <div class="card-body gap-5">
      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Global Name</legend>
        <input v-model="contract.name" class="input input-bordered w-full" type="text" required />
      </fieldset>
      <fieldset class="fieldset p-0 border-none">
        <legend class="fieldset-legend">Base Description</legend>
        <textarea v-model="contract.description" class="textarea textarea-bordered w-full h-24" required />
      </fieldset>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Contract } from '@/models/contract/contract'
defineProps<{
  contract: Contract
}>()
</script>
