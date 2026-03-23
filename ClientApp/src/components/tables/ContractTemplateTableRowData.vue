<script setup lang="ts">
import type { ContractTemplate } from '../../models/contract-template';

defineProps<{
    template: ContractTemplate
}>()
</script>

<template>
    <td class="pl-8">{{ template.did }}</td>
    <td class="pl-8">{{ template.document_number }}</td>
    <td class="pl-8">{{ template.name }}</td>
    <td class="pl-8">{{ template.state }}</td>
    <td class="pl-8">{{ template.version }}</td>
    <td class="pl-8">{{ template.created_by }}</td>
    <td class="pl-8">{{ new Date(template.created_at).toLocaleDateString() }}</td>
    <td class="pl-8">{{ template.description }}</td>
    <td class="pl-8">{{ template.template_data }}</td>
</template>
