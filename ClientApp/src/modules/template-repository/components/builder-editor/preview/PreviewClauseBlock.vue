<template>
  <template v-for="(seg, index) in segments" :key="index">
    <PreviewTextBlock v-if="seg.type === 'text'" :text="seg.value" />
    <PreviewParamInput v-else-if="seg.type === 'param'" :type="seg.paramType" :label="seg.label" />
    <span v-else-if="seg.type === 'newline'" :class="previewNewlineSpanClass" aria-hidden="true" />
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SemanticCondition, SemanticParameterType } from '@template-repository/models/contract-templace'
import { parseSegments, isText, isPlaceholder, type Segment, isNewline } from '@template-repository/composables/useClauseTextChips'
import PreviewParamInput from './PreviewParamInput.vue'
import PreviewTextBlock from './PreviewTextBlock.vue'
import { PREVIEW_NEWLINE_SPAN_CLASS } from './preview-classes'

const props = defineProps<{
  text: string
  semanticConditions: SemanticCondition[]
}>()

type PreviewSegment =
  | { type: 'text'; value: string }
  | { type: 'param'; paramType: SemanticParameterType; label: string }
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
        paramType,
        label: seg.parameterName,
      })
    } else if (isNewline(seg)) {
      result.push({ type: 'newline' })
    }
  }
  return result
})
</script>
