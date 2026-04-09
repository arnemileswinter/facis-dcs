<template>
  <span class="tooltip tooltip-top inline-flex items-baseline" :data-tip="label">
    <input v-if="type === 'string'" v-model="stringValue" type="text" @input="emitStringValue"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'decimal' || type === 'integer'" v-model="numberValue" type="number" @input="emitNumberValue"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'date'" v-model="dateValue" type="date" @input="emitDateValue"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
  </span>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { SemanticParameterType } from '@template-repository/models/contract-templace'

const props = defineProps<{
  type: SemanticParameterType
  label?: string
  value?: string | number
}>()
const emit = defineEmits<{
  (e: 'update:value', value: string | number): void
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

watch(
  () => props.value,
  (value) => {
    const next = value ?? ''
    if (props.type === 'string') stringValue.value = `${next}`
    if (props.type === 'decimal' || props.type === 'integer') numberValue.value = `${next}`
    if (props.type === 'date') dateValue.value = `${next}`
  },
  { immediate: true },
)

function emitStringValue(event: Event) {
  const next = (event.target as HTMLInputElement | null)?.value ?? ''
  emit('update:value', next)
}

function emitNumberValue(event: Event) {
  const next = (event.target as HTMLInputElement | null)?.value ?? ''
  if (next === '') {
    emit('update:value', '')
    return
  }
  const parsed = Number(next)
  emit('update:value', Number.isNaN(parsed) ? '' : parsed)
}

function emitDateValue(event: Event) {
  const next = (event.target as HTMLInputElement | null)?.value ?? ''
  emit('update:value', next)
}
</script>
