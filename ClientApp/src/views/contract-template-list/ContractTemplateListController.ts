import { onMounted, ref, type Ref } from 'vue'
import type { PartialContractTemplate } from '../../models/contract-template'
import { ContractTemplateService } from '../../services/contract-template-service'

export function useTemplateTable() {
    const templates: Ref<PartialContractTemplate[]> = ref([])
    const loading = ref(true)
    const error = ref('')

    const loadTemplates = async () => {
        loading.value = true
        error.value = ''
        try {
            const data = await ContractTemplateService.retrieve()
            console.log(data)
            templates.value = data

        } catch (err: any) {
            error.value = err.message || 'Fehler beim Laden der Templates'
        } finally {
            loading.value = false
        }
    }

    const refresh = () => loadTemplates()  // Für manuelles Refresh

    const getTemplateById = async (did: string, version: number, document_number: number) => {
        try {
            return await ContractTemplateService.retrieveById({ did, version, document_number })
        } catch (err: any) {
            console.error('Template konnte nicht geladen werden:', err)
            return null
        }
    }
    onMounted(loadTemplates)

    return {
        templates,
        loading,
        error,
        loadTemplates,
        refresh,
        getTemplateById
    }
}
