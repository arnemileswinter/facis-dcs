import type { ReviewTaskState } from "@/types/review-task-state"

export interface ContractTemplateReviewTask {
    did: string
    document_number: number
    version: number
    state: ReviewTaskState
    reviewer: string
    created_at: string
}
