<template>
  <span class="tooltip tooltip-top inline-flex items-baseline" :data-tip="label">
    <input v-if="type === 'string'" v-model="stringValue" type="text" @input="emitStringValue"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'integer'" v-model="numberValue" type="text" inputmode="numeric" @keydown="onIntegerKeyDown" @input="emitIntegerValue"
      class="border-b border-base-400 bg-transparent text-sm leading-relaxed px-0.5 outline-none" :aria-label="label" />
    <input v-else-if="type === 'decimal'" v-model="numberValue" type="number" @input="emitDecimalValue"
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

function emitIntegerValue(event: Event) {
  const next =  getIntegerInput((event.target as HTMLInputElement | null)?.value ?? '')
  if (next === '' || next === '-') {
    emit('update:value', '')
    return
  }
  const parsed = Number(next)
  emit('update:value', Number.isInteger(parsed) ? parsed : '')
}

function emitDecimalValue(event: Event) {
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

function getIntegerInput(value: string): string {
  if (!value) return ''
  const trimmed = value.trim()
  const negative = trimmed.startsWith('-')
  const digitsOnly = trimmed.replace(/[^\d]/g, '')
  if (!digitsOnly) return negative ? '-' : ''
  return `${negative ? '-' : ''}${digitsOnly}`
}

function onIntegerKeyDown(event: KeyboardEvent) {
  const allowedControlKeys = new Set([
    'Backspace', 'Delete', 'Tab', 'Escape', 'Enter', 'ArrowLeft', 'ArrowRight', 'Home', 'End',
  ])
  if (allowedControlKeys.has(event.key) || event.metaKey || event.ctrlKey) return
  if (event.key === '-') {
    const input = event.target as HTMLInputElement | null
    const cursorAtStart = (input?.selectionStart ?? 0) === 0
    const hasMinus = numberValue.value.includes('-')
    const allSelected = input?.selectionStart === 0 && input?.selectionEnd === input?.value.length
    if ((cursorAtStart || allSelected) && !hasMinus) return
    event.preventDefault()
    return
  }
  if (event.key.length === 1 && !/\d/.test(event.key)) {
    event.preventDefault()
  }
}
</script>
