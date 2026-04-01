import type { PartialContractTemplate } from '@/models/contract-template'
import type { ContractTemplateApprovalTask } from '@/models/contract-template-approval-task'
import type { ContractTemplateReviewTask } from '@/models/contract-template-review-task'
import { ROUTES } from '@/router/router'
import { contractTemplateService } from '@/services/contract-template-service'
import { useAuthStore } from '@/stores/auth-store'
import { useContractTemplatesStore } from '@/stores/contract-templates-store'
import { useDataRouteStore } from '@/stores/data-route-store'
import type { UserRole } from '@/types/user-role'
import { onMounted, ref, type Ref } from 'vue'

export function useTemplateTable() {
    const templatesStore = useContractTemplatesStore()
    const templates: Ref<PartialContractTemplate[]> = ref([])
    const reviewTasks: Ref<ContractTemplateReviewTask[]> = ref([])
    const approvalTasks: Ref<ContractTemplateApprovalTask[]> = ref([])
    const roles: Ref<UserRole[]> = ref([])
    const loading = ref(true)
    const error = ref('')
    const authStore = useAuthStore()
    const dataRouteStore = useDataRouteStore()

    const loadTemplates = async () => {
        loading.value = true
        error.value = ''
        try {
            const data = await contractTemplateService.retrieve()
            templates.value = data.contract_templates
            templatesStore.contractTemplates = templates.value
            reviewTasks.value = data.review_tasks
            templatesStore.reviewTasks = reviewTasks.value
            approvalTasks.value = data.approval_tasks
            templatesStore.approvalTasks = approvalTasks.value
            roles.value = authStore.user?.roles ?? []
            if (reviewTasks.value.length > 0) {
                dataRouteStore.addDataRouteLoaded(ROUTES.TASKS.REVIEWS)
            }
            if (approvalTasks.value.length > 0) {
                dataRouteStore.addDataRouteLoaded(ROUTES.TASKS.APPROVALS)
            }
        } catch (err: any) {
            error.value = err.message || 'Fehler beim Laden der Templates'
        } finally {
            loading.value = false
        }
    }

    const refresh = () => loadTemplates()  // Für manuelles Refresh

    const getTemplateById = async (did: string) => {
        try {
            return await contractTemplateService.retrieveById({ did })
        } catch (err: any) {
            console.error('Template konnte nicht geladen werden:', err)
            return null
        }
    }

    onMounted(loadTemplates)

    const hasReviewTask = (template: PartialContractTemplate): boolean => {
        const currentUser = authStore.user
        if (!currentUser) return false
        return reviewTasks.value.some((task) => {
            const isDidMatch = task.did === template.did
            const isVersionMatch = !template.version || task.version === template.version
            const isDocumentNumberMatch = !template.document_number || task.document_number === template.document_number
            return (
                isDidMatch &&
                isVersionMatch &&
                isDocumentNumberMatch &&
                task.reviewer === currentUser.username
            )
        })
    }

    const hasApprovalTask = (template: PartialContractTemplate): boolean => {
        const currentUser = authStore.user
        if (!currentUser) return false
        return approvalTasks.value.some((task) => {
            const isDidMatch = task.did === template.did
            const isVersionMatch = !template.version || task.version === template.version
            const isDocumentNumberMatch = !template.document_number || task.document_number === template.document_number
            return (
                isDidMatch &&
                isVersionMatch &&
                isDocumentNumberMatch &&
                task.approver === currentUser.username
            )
        })
    }

    return {
        templates,
        reviewTasks,
        approvalTasks,
        roles,
        loading,
        error,
        loadTemplates,
        refresh,
        getTemplateById,
        hasReviewTask,
        hasApprovalTask,
    }
}
