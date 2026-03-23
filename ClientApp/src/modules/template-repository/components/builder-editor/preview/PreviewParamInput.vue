<template>
  <span class="tooltip tooltip-top inline-flex items-baseline" :data-tip="label">
    <input v-if="type === 'string'" v-model="stringValue" type="text"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'decimal' || type === 'integer'" v-model="numberValue" type="number"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'date'" v-model="dateValue" type="date"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
  </span>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { SemanticParameterType } from '@template-repository/models/contract-templace'

const props = defineProps<{
  type: SemanticParameterType
  label?: string
}>()

const stringValue = ref('')
const numberValue = ref('')
const dateValue = ref('')

watch(
  () => props.type,
  () => {
    stringValue.value = ''
    numberValue.value = ''
    dateValue.value = ''
  }
)
</script>
