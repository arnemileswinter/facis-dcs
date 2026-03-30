export type ReviewTaskState = (typeof ContractTemplateReviewTaskState)[keyof typeof ContractTemplateReviewTaskState]

export const ContractTemplateReviewTaskState = {
  open: 'OPEN',
  rejected: 'REJECTED',
  verified: 'VERIFIED',
  approved: 'APPROVED',
} as const

export const reviewTaskStates: ReviewTaskState[] = Object.values(ContractTemplateReviewTaskState)
