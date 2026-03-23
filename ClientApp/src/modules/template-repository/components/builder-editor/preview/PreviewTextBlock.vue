<template>
  <template v-for="(line, index) in lines" :key="index">
    <span :class="previewTextClass">{{ line }}</span>
    <span v-if="index < lines.length - 1" :class="previewNewlineSpanClass" aria-hidden="true" />
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  PREVIEW_TEXT_CLASS,
  PREVIEW_NEWLINE_SPAN_CLASS,
} from './preview-classes'

const props = defineProps<{
  text: string
}>()

const lines = computed(() => (props.text ?? '').split('\n'))

const previewTextClass = PREVIEW_TEXT_CLASS
const previewNewlineSpanClass = PREVIEW_NEWLINE_SPAN_CLASS
</script>
