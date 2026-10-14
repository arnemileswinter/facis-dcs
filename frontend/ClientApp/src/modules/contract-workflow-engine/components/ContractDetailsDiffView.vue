<script setup lang="ts">
import { computed } from 'vue'
import type { ContractDetailsSnapshot } from '@contract-workflow-engine/models/contract-details-snapshot'

const props = withDefaults(
  defineProps<{
    left?: ContractDetailsSnapshot | null
    right?: ContractDetailsSnapshot | null
    leftPaneTitle?: string
    rightPaneTitle?: string
    leftTestId?: string
    rightTestId?: string
    highlightDiff?: boolean
  }>(),
  {
    leftPaneTitle: 'Left',
    rightPaneTitle: 'Right',
    leftTestId: 'contract-details-diff-left',
    rightTestId: 'contract-details-diff-right',
    highlightDiff: true,
  },
)

type DetailKey = keyof ContractDetailsSnapshot

const rows: { key: DetailKey; label: string }[] = [
  { key: 'name', label: 'Title' },
  { key: 'contract_version', label: 'Version' },
  { key: 'description', label: 'Description' },
  { key: 'start_date', label: 'Start date' },
  { key: 'exp_date', label: 'Expiration date' },
  { key: 'exp_notice_period', label: 'Notice period' },
  { key: 'exp_policy', label: 'Expiry policy' },
]

function displayValue(snapshot: ContractDetailsSnapshot | null | undefined, key: DetailKey): string {
  const value = snapshot?.[key]
  if (value == null || value === '') return ''
  if (key === 'start_date' || key === 'exp_date') {
    const date = new Date(String(value))
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString()
  }
  return String(value)
}

function valuesDiffer(key: DetailKey): boolean {
  if (!props.highlightDiff) return false
  const left = props.left?.[key]
  const right = props.right?.[key]
  return String(left ?? '') !== String(right ?? '')
}

const fieldRows = computed(() =>
  rows.map((row) => ({
    ...row,
    left: displayValue(props.left, row.key),
    right: displayValue(props.right, row.key),
    changed: valuesDiffer(row.key),
  })),
)
</script>

<template>
  <div data-testid="contract-details-diff" class="grid grid-cols-1 gap-4 lg:grid-cols-2">
    <article :data-testid="leftTestId" class="card border border-base-300 bg-base-100 shadow-sm">
      <div class="card-body min-w-0 p-4">
        <div class="mb-2 text-sm font-semibold text-base-content/70">{{ leftPaneTitle }}</div>
        <dl
          class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-base leading-6 text-base-content/90"
        >
          <template v-for="row in fieldRows" :key="`L-${row.key}`">
            <dt class="text-base-content/70">{{ row.label }}</dt>
            <dd class="m-0 break-words whitespace-pre-wrap" :class="{ 'bg-amber-100/20 px-1': row.changed }">
              {{ row.left }}
            </dd>
          </template>
        </dl>
      </div>
    </article>
    <article :data-testid="rightTestId" class="card border border-base-300 bg-base-100 shadow-sm">
      <div class="card-body min-w-0 p-4">
        <div class="mb-2 text-sm font-semibold text-base-content/70">{{ rightPaneTitle }}</div>
        <dl
          class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-base leading-6 text-base-content/90"
        >
          <template v-for="row in fieldRows" :key="`R-${row.key}`">
            <dt class="text-base-content/70">{{ row.label }}</dt>
            <dd class="m-0 break-words whitespace-pre-wrap" :class="{ 'bg-amber-100/20 px-1': row.changed }">
              {{ row.right }}
            </dd>
          </template>
        </dl>
      </div>
    </article>
  </div>
</template>
