<template>
  <div class="flex justify-between mb-8">
    <h2 class="text-2xl/7 font-bold sm:truncate sm:text-3xl sm:tracking-tight">
      {{ $route.meta.name }}
    </h2>

    <RouterLink
      :to="{ name: 'templates.new' }"
      class="btn rounded-box self-end btn-secondary gap-2"
      #default="{ route }"
    >
      {{ route.meta.name }}
    </RouterLink>
  </div>
  <div>
    <div v-if="loading">Lade Templates...</div>
    <div v-else-if="error">{{ error }}</div>
    <div v-else>
      <TemplateList :items="templates" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useTemplateTable } from './ContractTemplateListController'
import TemplateList from '../../components/lists/template-list/TemplateList.vue'

const { templates, loading, error } = useTemplateTable()
</script>
