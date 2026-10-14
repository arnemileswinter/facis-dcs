<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'

const store = useDcsDraftStore()
const { did, created_by, created_at, updated_at } = storeToRefs(store)

function formatWhen(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

const rows = computed(() => [
  { label: 'Template ID', value: did.value ?? '—' },
  { label: 'creator', value: created_by.value !== '' ? created_by.value : '—' },
  { label: 'creation timestamp', value: formatWhen(created_at.value) },
  { label: 'last change timestamp', value: formatWhen(updated_at.value) },
])
</script>

<template>
  <dl
    data-testid="template-provenance"
    class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 rounded-box border border-base-content/10 bg-base-200/40 p-4 text-sm"
  >
    <template v-for="row in rows" :key="row.label">
      <dt class="font-medium text-base-content/70">{{ row.label }}</dt>
      <dd class="min-w-0 break-all">{{ row.value }}</dd>
    </template>
  </dl>
</template>
