<template>
  <span
    class="clause-placeholder-slot inline-block border-b-2 border-dashed border-primary/50 bg-primary/5 text-primary px-1 rounded-sm font-medium cursor-default">
    {{ label }}
  </span>
</template>

<script setup lang="ts">
defineProps<{
  label: string
}>()
</script>
