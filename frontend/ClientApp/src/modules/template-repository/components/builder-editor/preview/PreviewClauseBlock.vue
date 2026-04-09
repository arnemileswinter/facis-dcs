<template>
  <template v-for="(seg, index) in segments" :key="index">
    <PreviewTextBlock v-if="seg.type === 'text'" :text="seg.value" />
    <PreviewParamInput
      v-else-if="seg.type === 'param'"
      :type="seg.paramType"
      :label="seg.label"
      :value="seg.value"
      @update:value="(val) => onParamValueChange(seg, val)"
    />
    <span v-else-if="seg.type === 'newline'" :class="previewNewlineSpanClass" aria-hidden="true" />
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SemanticConditionValue } from '@/models/contract-data'
import type { SemanticCondition, SemanticParameterType } from '@template-repository/models/contract-templace'
import { parseSegments, isText, isPlaceholder, type Segment, isNewline } from '@template-repository/composables/useClauseTextChips'
import type { SemanticConditionValueSetter } from '@/modules/contract-workflow-engine/models/contract-content-values-store'
import PreviewParamInput from './PreviewParamInput.vue'
import PreviewTextBlock from './PreviewTextBlock.vue'
import { PREVIEW_NEWLINE_SPAN_CLASS } from './preview-classes'

const props = defineProps<{
  blockId: string
  subBlockId?: string
  text: string
  semanticConditions: SemanticCondition[]
  semanticConditionValues?: SemanticConditionValue[]
  setSemanticConditionValue?: SemanticConditionValueSetter
}>()

type PreviewSegment =
  | { type: 'text'; value: string }
  | { type: 'param'; conditionId: string; parameterName: string; paramType: SemanticParameterType; label: string; value?: string | number }
  | { type: 'newline' }

const previewNewlineSpanClass = PREVIEW_NEWLINE_SPAN_CLASS

const segments = computed<PreviewSegment[]>(() => {
  const normalizedText = (props.text ?? '').replace(/^[\s\u00A0]+/, '')
  const baseSegments: Segment[] = parseSegments(normalizedText, props.semanticConditions)
  const result: PreviewSegment[] = []
  for (const seg of baseSegments) {
    if (isText(seg)) {
      result.push({ type: 'text', value: seg.value })
    } else if (isPlaceholder(seg)) {
      const cond = props.semanticConditions.find((c) => c.conditionId === seg.conditionId)
      const param = cond?.parameters.find((p) => p.parameterName === seg.parameterName)
      const paramType: SemanticParameterType = param?.type ?? 'string'
      result.push({
        type: 'param',
        conditionId: seg.conditionId,
        parameterName: seg.parameterName,
        paramType,
        label: seg.parameterName,
        value: findSemanticValue(seg.conditionId, seg.parameterName),
      })
    } else if (isNewline(seg)) {
      result.push({ type: 'newline' })
    }
  }
  return result
})

function onParamValueChange(seg: PreviewSegment, value: string | number) {
  if (seg.type !== 'param') return
  props.setSemanticConditionValue?.(props.blockId, seg.conditionId, seg.parameterName, value, props.subBlockId)
}

function findSemanticValue(conditionId: string, parameterName: string): string | number | undefined {
  return props.semanticConditionValues?.find((item) => {
    const sameSub = (item.subBlockId ?? undefined) === (props.subBlockId ?? undefined)
    return (
      item.blockId === props.blockId &&
      sameSub &&
      item.conditionId === conditionId &&
      item.parameterName === parameterName
    )
  })?.parameterValue
}
</script>
