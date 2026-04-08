import type { NegotiationTaskState } from "@/types/negotiation-task-state"

export interface ContractNegotiationTask {
  did: string
  contract_version?: number
  state: NegotiationTaskState
  negotiator: string
  created_at: string
}
