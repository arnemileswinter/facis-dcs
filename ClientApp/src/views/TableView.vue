<script setup lang="ts">
import DataTable from '../components/tables/DataTable.vue'
import type { TableItem } from '../models/table-item'

const tableItems: TableItem[] = [
  { id: 12, name: 'Tester' },
  { id: 22, name: 'Tester 2' },
  { id: 32, name: 'Tester 3' },
]
</script>

<template>
  <DataTable :items="tableItems" />
</template>
