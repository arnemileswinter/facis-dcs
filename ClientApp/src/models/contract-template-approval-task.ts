import type { ApprovalTaskState } from "@/types/approval-task-state"

export interface ContractTemplateApprovalTask {
    did: string
    document_number: number
    version: number
    state: ApprovalTaskState
    approver: string
    created_at: string
}
