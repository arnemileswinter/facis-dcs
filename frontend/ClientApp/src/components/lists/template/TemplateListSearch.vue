<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { contractTemplateService } from '@/services/contract-template-service'
import { type PartyType, partyTypes } from '@/types/party-type'
import ListSearch from '../ListSearch.vue'
import type { PartialContractTemplate } from '@/models/contract-template/contract-template'
import type { ContractTemplateSearchResponse } from '@/models/responses/template-response'

defineProps<{
  templates: PartialContractTemplate[]
}>()

const partyType = ref<PartyType | ''>('')
const partyTypeId = useId()
const filters = computed(() => ({ party_type: partyType.value || undefined }))
const emit = defineEmits<{
  searchResult: [value: PartialContractTemplate[] | null]
}>()

const filterLabels: Partial<Record<keyof PartialContractTemplate, string>> = {
  did: 'DID',
  name: 'Name',
  description: 'Description',
  version: 'Version',
  template_data: 'Template Data',
}

const emptyTemplate: PartialContractTemplate = {
  did: '',
  version: 1,
  created_at: '',
  updated_at: '',
  name: '',
  template_type: 'CONTRACT_TEMPLATE',
  party_type: 'SINGLE_PARTY',
  state: 'DRAFT',
  created_by: '',
}

const responseMapper = (response: ContractTemplateSearchResponse) =>
  response.map((item) => ({
    did: item.did,
    name: item.name,
    description: item.description,
    version: parseInt(item.version, 10),
    state: item.state,
    updated_at: item.updated_at,
    created_at: item.created_at,
    template_type: item.template_type,
    party_type: item.party_type,
    created_by: item.created_by,
  }))
</script>

<template>
  <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center [&>.join]:m-0">
    <div class="shrink-0 text-sm">
      <label :for="partyTypeId" class="sr-only">Party type</label>
      <select :id="partyTypeId" v-model="partyType" class="select w-max min-w-44">
        <option value="">All party types</option>
        <option v-for="option in partyTypes" :key="option.value" :value="option.value">{{ option.label }}</option>
      </select>
    </div>
    <ListSearch
      class="w-full min-w-0 sm:w-auto sm:max-w-full sm:flex-1"
      :items="templates"
      :filter-labels="filterLabels"
      :filters="filters"
      :empty-item="emptyTemplate"
      :search-fn="async (request) => responseMapper(await contractTemplateService.search(request))"
      placeholder="Search templates"
      @search-result="(result) => emit('searchResult', result)"
    />
  </div>
</template>
