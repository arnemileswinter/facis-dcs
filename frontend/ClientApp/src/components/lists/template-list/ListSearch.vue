<script setup lang="ts">
import type { PartialContractTemplate } from '@/models/contract-template'
import type { ContractTemplateSearchResponse } from '@/models/responses/template-response'
import { contractTemplateService } from '@/services/contract-template-service'
import { Combobox, ComboboxInput, ComboboxOption, ComboboxOptions } from '@headlessui/vue'
import { computed, ref, useTemplateRef, type Ref } from 'vue'

const props = defineProps<{
  items: PartialContractTemplate[]
}>()

const emit = defineEmits<{
  searchResult: [value: PartialContractTemplate[]]
}>()

const searchQuery = ref('')
const isSearching = ref(false)

const filterLabels = {
  // did: 'DID',
  document_number: 'Document number',
  version: 'Version',
  name: 'Name',
  description: 'Description',
  // filter: 'Filter',
} as const
type FilterLabels = typeof filterLabels
type FilterLabelKey = keyof FilterLabels
type FilterLabelValue = FilterLabels[FilterLabelKey]

const empyt: PartialContractTemplate = {
  did: '12',
  document_number: '-1',
  version: -1,
  created_at: '2',
  updated_at: '2',
  name: '',
  template_type: 'FRAME_CONTRACT',
  state: 'DRAFT',
  created_by: '',
}

const selectedFilter = ref<FilterLabelValue>(filterLabels.name)
const filterPopover = useTemplateRef('filterPopover')
const searchResults: Ref<ContractTemplateSearchResponse> = ref([])

const selectedOption: Ref<PartialContractTemplate | null> = ref(null)

const searchKey = computed(() => {
  return (Object.keys(filterLabels) as FilterLabelKey[]).find((key) => filterLabels[key] === selectedFilter.value)
})

const searchedItems = computed(() => {
  if (searchQuery.value.length < 1) return props.items

  if (searchResults.value.length === 0) return []

  const backendIds = new Set(searchResults.value.map((item) => `${item.did}|${item.document_number}|${item.version}`))

  return props.items.filter((item) => backendIds.has(`${item.did}|${item.document_number}|${item.version}`))
})

const inputValue: Ref<PartialContractTemplate> = computed(() => {
  return searchQuery.value.length < 1 || !searchKey.value ? empyt : { ...empyt, [searchKey.value]: searchQuery.value }
})

async function searchRequest() {
  if (searchQuery.value.length < 1 || !searchKey.value) {
    searchResults.value = []
    return
  }

  isSearching.value = true
  try {
    await retrieveSearch()
  } finally {
    isSearching.value = false
  }
}

async function retrieveSearch() {
  if (!searchKey.value) return
  const request = { [searchKey.value]: searchQuery.value }
  const result = await contractTemplateService.search(request)
  searchResults.value = result
}

async function searchList(event?: Event) {
  if (event && event.target instanceof HTMLInputElement) {
    if (event.target.value !== searchQuery.value) {
      await searchRequest()
    }
  }
  emit('searchResult', searchedItems.value)
}

const getDisplayValue = (template: PartialContractTemplate | null): string => {
  return searchKey.value && template ? String(template[searchKey.value]) : ''
}

const autocompleteOptionClasses = (active: boolean, selected: boolean) => [
  'cursor-pointer px-4 py-2',
  active ? 'bg-secondary text-secondary-content' : 'bg-base-100',
  selected ? 'font-bold' : '',
]

async function onComboboxFocus() {
  await searchRequest()
}

function onSearchChange(event: Event) {
  searchQuery.value = (event.target as HTMLInputElement).value
  searchRequest()
}

function onComboboxUpdate(item: PartialContractTemplate) {
  selectedOption.value = item
  if (selectedOption.value) {
    searchQuery.value = searchKey.value ? String(selectedOption.value[searchKey.value]) : ''
  }
}

function onFilterSelect(label: FilterLabelValue) {
  selectedFilter.value = label
  filterPopover.value?.hidePopover()
}
</script>

<template>
  <div class="join m-2 flex-col sm:flex-row">
    <div class="join-item">
      <button
        id="list-btn-search"
        type="button"
        class="select select-secondary w-full rounded-t-md rounded-b-none sm:rounded-l-md sm:rounded-tr-none"
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
            <a :class="{ 'bg-primary text-primary-content': label === selectedFilter }" @click="onFilterSelect(label)">
              {{ label }}
            </a>
          </li>
        </template>
      </ul>
    </div>
    <div class="relative grow">
      <Combobox v-model="selectedOption" @update:model-value="onComboboxUpdate" nullable>
        <label class="input input-secondary join-item w-full rounded-none -mt-px ms-0 sm:mt-0 sm:-ms-px">
          <ComboboxInput
            @change="onSearchChange"
            @focus="onComboboxFocus"
            @keydown.enter="searchList"
            :display-value="(template) => getDisplayValue(template as PartialContractTemplate | null)"
            placeholder="Search templates"
            class="w-full bg-transparent"
          />
        </label>

        <ComboboxOptions
          v-if="searchQuery.length > 0"
          class="absolute left-0 right-0 top-full z-10 rounded-lg border border-base-300 bg-base-100 shadow-lg"
        >
          <ComboboxOption :value="inputValue" class="hidden"></ComboboxOption>

          <div v-if="isSearching" class="px-4 py-2 text-base-content/50">Searching...</div>
          <template v-else-if="searchedItems.length > 0">
            <ComboboxOption
              v-for="item in searchedItems"
              :key="`${item.did}|${item.document_number}|${item.version}`"
              :value="item"
              as="template"
              v-slot="{ active, selected }"
            >
              <li v-if="searchKey" :class="autocompleteOptionClasses(active, selected)">
                <span class="block truncate">{{ item[searchKey] }}</span>
              </li>
            </ComboboxOption>
          </template>

          <div v-else class="px-4 py-2 text-base-content/50">No templates found</div>
        </ComboboxOptions>
      </Combobox>
    </div>
    <button
      @click="searchList"
      class="btn btn-secondary join-item rounded-b-md rounded-t-none sm:rounded-r-md sm:rounded-bl-none -mt-px ms-0 sm:mt-0 sm:-ms-px"
    >
      Search
    </button>
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
