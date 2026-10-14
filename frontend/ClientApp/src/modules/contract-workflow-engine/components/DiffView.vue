<script setup lang="ts">
import { computed } from 'vue'
import DiffPane from '@contract-workflow-engine/components/diff-view/DiffPane.vue'
import { useContractBlockDiff } from '@contract-workflow-engine/composables/useContractBlockDiff'
import {
  type ContractPlainTextBlock,
  useContractPlainTextConverter,
} from '@contract-workflow-engine/composables/useContractPlainTextConverter'
import type { ContractData } from '@/models/contract/contract-data'
import type { ContractDetailsSnapshot } from '@contract-workflow-engine/models/contract-details-snapshot'

const props = withDefaults(
  defineProps<{
    leftContractData?: ContractData
    rightContractData?: ContractData
    leftDetails?: ContractDetailsSnapshot | null
    rightDetails?: ContractDetailsSnapshot | null
    showLineNumbers?: boolean
    highlightDiff?: boolean
    leftPaneTitle?: string
    rightPaneTitle?: string
    leftTestId?: string
    rightTestId?: string
  }>(),
  {
    showLineNumbers: true,
    highlightDiff: true,
    leftPaneTitle: '',
    rightPaneTitle: '',
    leftDetails: null,
    rightDetails: null,
    leftTestId: undefined,
    rightTestId: undefined,
  },
)

const { convertContractToPlainTextBlocks } = useContractPlainTextConverter()
const { buildContractBlockDiff } = useContractBlockDiff()

const hasLeftContractData = computed(() => !!props.leftContractData)

const leftBlocks = computed<ContractPlainTextBlock[]>(() => {
  if (!props.leftContractData) return []
  return convertContractToPlainTextBlocks(props.leftContractData)
})

const rightBlocks = computed<ContractPlainTextBlock[]>(() => {
  if (!props.rightContractData) return []
  return convertContractToPlainTextBlocks(props.rightContractData)
})

const contractDiffDocument = computed(() => buildContractBlockDiff(leftBlocks.value, rightBlocks.value))

const detailKeys: (keyof ContractDetailsSnapshot)[] = [
  'name',
  'contract_version',
  'description',
  'start_date',
  'exp_date',
  'exp_notice_period',
  'exp_policy',
]

const changedDetailKeys = computed(() => {
  if (!props.highlightDiff) return [] as (keyof ContractDetailsSnapshot)[]
  return detailKeys.filter((key) => String(props.leftDetails?.[key] ?? '') !== String(props.rightDetails?.[key] ?? ''))
})
</script>

<template>
  <div class="grid min-h-128 grid-cols-1 gap-4 lg:grid-cols-2">
    <DiffPane
      :title="leftPaneTitle"
      :blocks="leftBlocks"
      :diff-rows="contractDiffDocument.leftRows"
      :highlight-diff="highlightDiff"
      :show-no-prior-version="!hasLeftContractData && !leftDetails"
      :show-line-numbers="showLineNumbers"
      :details="leftDetails"
      :changed-detail-keys="changedDetailKeys"
      :test-id="leftTestId"
    />
    <DiffPane
      :title="rightPaneTitle"
      :blocks="rightBlocks"
      :diff-rows="contractDiffDocument.rightRows"
      :highlight-diff="highlightDiff"
      :show-line-numbers="showLineNumbers"
      :details="rightDetails"
      :changed-detail-keys="changedDetailKeys"
      :test-id="rightTestId"
    />
  </div>
</template>
