export type ApprovalTaskState =
  (typeof ContractTemplateApprovalTaskState)[keyof typeof ContractTemplateApprovalTaskState]

export type ContractApprovalTaskState = Omit<typeof ContractTemplateApprovalTaskState, 'resubmitted'>

export const ContractTemplateApprovalTaskState = {
  open: 'OPEN',
  rejected: 'REJECTED',
  resubmitted: 'RESUBMITTED',
  approved: 'APPROVED',
} as const

export const approvalTaskStates: ApprovalTaskState[] = Object.values(ContractTemplateApprovalTaskState)
