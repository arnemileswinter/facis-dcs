export type ContractState = (typeof ContractState)[keyof typeof ContractState]
export const ContractState = {
  draft: 'DRAFT',
  negotiation: 'NEGOTIATION',
  submitted: 'SUBMITTED',
  reviewed: 'REVIEWED',
  approved: 'APPROVED',
  deleted: 'DELETED',
  deprecated: 'DEPRECATED',
} as const
