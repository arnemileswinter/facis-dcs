import type { ContractState } from "@/types/contract-state"
import type { Contract } from "../contract/contract"
import type { ContractApprovalTask } from "../contract/contract-approval-task"
import type { ContractReviewTask } from "../contract/contract-review-task"
import type { ContractNegotiation } from "../contract/contract-negotiation"

export interface ContractCreateResponse {
  did: string
}

export interface ContractUpdateResponse {
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
