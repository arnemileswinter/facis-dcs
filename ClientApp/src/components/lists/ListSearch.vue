<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import { ContractTemplateService } from '@/services/contract-template-service'
import { computed, ref, useTemplateRef, type Ref } from 'vue'

const props = defineProps<{
  items: PartialContractTemplate[]
}>()

const emit = defineEmits<{
  searchResult: [value: PartialContractTemplate[]]
}>()

const search = ref('')

const filterLabels = {
  did: 'DID',
  document_number: 'Document number',
  version: 'Version',
  template_type: 'Template type',
  state: 'State',
  name: 'Name',
  description: 'Description',
  filter: 'Filter',
} as const
type FilterLabels = typeof filterLabels
type FilterLabelKey = keyof FilterLabels
type FilterLabelValue = FilterLabels[FilterLabelKey]

const selectedFilter = ref<FilterLabelValue>('Name')

const filterPopover = useTemplateRef('filterPopover')

const searchResults: Ref<Set<string>> = ref(new Set())

const searchKey = computed(() => {
  return (Object.keys(filterLabels) as FilterLabelKey[]).find((key) => filterLabels[key] === selectedFilter.value)
})

const searchedItems = computed(() => {
  if (search.value.length < 1) return props.items
  return props.items.filter((item) => searchResults.value.has(`${item.did}|${item.document_number}|${item.version}`))
})

async function searchList() {
  if (search.value.length < 1 || !searchKey.value) {
    emit('searchResult', props.items)
    return
  }

  const request = { [searchKey.value]: search.value }
  const searchResult = await ContractTemplateService.search(request)
  searchResults.value = new Set(searchResult.map((item) => `${item.did}|${item.document_number}|${item.version}`))
  emit('searchResult', searchedItems.value)
}

function onFilterSelect(label: FilterLabelValue) {
  selectedFilter.value = label
  filterPopover.value?.hidePopover()
  searchList()
}
</script>

<template>
  <div class="join m-2">
    <div class="join-item">
      <button
        id="list-btn-search"
        type="button"
        class="select select-secondary join-item"
        popovertarget="list-popover-search"
      >
        {{ selectedFilter }}
      </button>
      <ul
        ref="filterPopover"
        class="dropdown dropdown-start menu w-52 rounded-box bg-base-300 shadow-sm"
        popover
        id="list-popover-search"
      >
        <li class="menu-title">
          <span class="menu-disabled pointer-events-none select-none">Select search filter</span>
        </li>
        <template v-for="[key, label] in Object.entries(filterLabels)" :key="key">
          <li>
            <a
              :class="{ 'bg-primary text-primary-content': label === selectedFilter }"
              @click="onFilterSelect(label)"
            >
              {{ label }}
            </a>
          </li>
        </template>
      </ul>
    </div>
    <label class="input input-secondary join-item grow">
      <input
        type="text"
        v-model="search"
        @keyup.enter="searchList"
        placeholder="Search templates"
        aria-label="Search templates"
      />
    </label>
    <button @click="searchList" class="btn btn-secondary join-item">Search</button>
  </div>
</template>

<style scoped>
#list-btn-search {
  anchor-name: --anchor-list-search;
}

#list-popover-search {
  position-anchor: --anchor-list-search;
}
</style>
