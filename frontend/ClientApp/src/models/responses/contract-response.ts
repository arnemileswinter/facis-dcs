import type { ContractState } from '@/types/contract-state'
import type { Contract } from '../contract/contract'
import type { ContractApprovalTask } from '../contract/contract-approval-task'
import type { ContractReviewTask } from '../contract/contract-review-task'
import type { ContractNegotiation } from '../contract/contract-negotiation'

export interface ContractCreateResponse {
  did: string
}

export interface ContractUpdateResponse {
  did: string
}

export interface ContractSubmitResponse {
  did: string
}

export interface ContractRetrieveResponse {
  contracts: Contract[]
  review_tasks: ContractReviewTask[]
  approval_tasks: ContractApprovalTask[]
}

export interface ContractRetrieveByIdResponse {
  did: string
  contract_version?: number
  state: ContractState
  name?: string
  description?: string
  created_by: string
  created_at: string
  updated_at: string
  /** The data of that contract */
  contract_data: unknown
  negotiations: ContractNegotiation[]
}

export interface ContractReviewResponse {
  did: string
}

export interface ContractVerifyResponse {
  did: string
  findings?: string[]
}

interface ContractSearchResponseItem {
  did: string
  contract_version?: number
  state: ContractState
  name?: string
  description?: string
  created_at: string
  updated_at: string
}

export type ContractSearchResponse = ContractSearchResponseItem[]

export interface ContractNegotiationResponse {
  did: string
}

export interface ContractNegotiationRespondResponse {
  id: string
}

export interface ContractApproveResponse {
  did: string
}

export interface ContractRejectResponse {
  did: string
}

export interface ContractStoreResponse {
  did: string
}

export interface ContractTerminateResponse {
  did: string
}

export interface ContractAuditResponse {
  did: string
}
