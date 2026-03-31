import type { ContractApprovalTaskState } from "@/types/approval-task-state";

export interface ContractApprovalTask {
  did: string;
  contract_version?: string;
  state: ContractApprovalTaskState;
  approver: string;
  created_at: string;
}
