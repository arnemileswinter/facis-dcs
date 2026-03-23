<script setup lang="ts" generic="T extends TableItem">
import { computed, ref } from 'vue'
import type { TableItem } from '../../models/table-item'
import Pagination from '../Pagination.vue'
import TableRow from './TableRow.vue'
import { BarsArrowDownIcon, BarsArrowUpIcon, ChevronUpDownIcon } from '@heroicons/vue/20/solid'

const { items, headers = [] } = defineProps<{
  items: T[]
  readonly headers?: string[]
}>()

const itemsPerPage = ref(4)

if (headers.length === 0) {
  headers.push('id', 'name')
}

const pages = computed(() => Math.ceil(items.length / itemsPerPage.value))
const currentPage = ref(1)
const sortBy = ref(headers[0]!)
const sortOrder = ref(1)

const itemsSorted = computed(() => {
  if (!headers.includes(sortBy.value)) {
    return items
  }
  return items.slice().sort((a, b) => {
    const result = a[sortBy.value as keyof T] > b[sortBy.value as keyof T] ? 1 : -1
    return sortOrder.value * result
  })
})

const itemsDisplayed = computed(() => {
  const from = (currentPage.value - 1) * itemsPerPage.value
  const to = from + itemsPerPage.value
  return itemsSorted.value.slice(from, to)
})

function handlePageChange(selectedPage: number) {
  currentPage.value = selectedPage
}

function sortItemsBy(item: string) {
  const sorter = headers.find((h) => h === item) ?? 'id'
  sortOrder.value = sortBy.value === sorter ? -sortOrder.value : 1
  sortBy.value = sorter
}
</script>

<template>
  <div class="m-4">
    <div class="overflow-x-auto rounded-box border border-base-content/5 bg-base-100">
      <table class="table m-4 w-full">
        <thead>
          <tr>
            <template v-for="header in headers">
              <th>
                <button
                  class="btn btn-ghost p-2"
                  :class="{ 'btn-outline': header === sortBy }"
                  @click="sortItemsBy(header)"
                  :aria-sort="header === sortBy ? (sortOrder === 1 ? 'ascending' : 'descending') : 'none'"
                >
                  <span>{{ header.toLocaleUpperCase('de') }}</span
                  ><ChevronUpDownIcon v-if="header !== sortBy" class="w-5 h-5" /><BarsArrowUpIcon
                    v-else-if="sortOrder === 1"
                    class="w-5 h-5"
                  /><BarsArrowDownIcon v-else class="w-5 h-5" />
                </button>
              </th>
            </template>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(item, i) in itemsDisplayed" :key="i">
            <TableRow :item="item" :hide-default="headers.length > 2">
              <template #extraCols="{ item }">
                <slot name="extraCols" :item="item"></slot>
              </template>
            </TableRow>
          </template>
        </tbody>
      </table>
    </div>
    <Pagination :pages="pages" @page-change="handlePageChange" />
  </div>
</template>
