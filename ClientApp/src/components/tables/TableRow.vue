<script setup lang="ts" generic="T extends TableItem">
import type { TableItem } from '../../models/table-item'

defineProps<{
  item: T
  hideDefault?: boolean
}>()
</script>

<template>
  <tr class="hover:bg-base-300">
    <template v-if="!hideDefault">
      <th class="pl-8">{{ item.id }}</th>
      <td class="pl-8">{{ item.name }}</td>
    </template>
    <slot name="extraCols" :item="item"></slot>
    <td class="pl-8">
      <button class="btn btn-warning btn-outline">Edit</button>
    </td>
  </tr>
</template>
