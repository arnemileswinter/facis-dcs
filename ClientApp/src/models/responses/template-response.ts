import type { ContractTemplateState } from '@/types/contract-template-state'
import type { TemplateType } from '@/types/template-type'
import type { PartialContractTemplate } from '../contract-template'
import type { ContractTemplateReviewTask } from '../contract-template-review-task'
import type { ContractTemplateApprovalTask } from '../contract-template-approval-task'

interface ContractTemplateBaseResponse {
  did: string
  document_number: number
  version: number
}

export interface ContractTemplateCreateResponse extends ContractTemplateBaseResponse {}

export interface ContractTemplateSubmitResponse extends ContractTemplateBaseResponse {}

export interface ContractTemplateUpdateResponse extends ContractTemplateBaseResponse {}

export interface ContractTemplateSearchResponseItem extends ContractTemplateBaseResponse {
  state: ContractTemplateState
  name?: string
  description?: string
  created_at: string
  updated_at: string
}

export interface ContractTemplateSearchResponse {
  search_results: ContractTemplateSearchResponseItem[]
}

export interface ContractTemplateRetrieveResponse {
  contract_templates: PartialContractTemplate[]
  review_tasks: ContractTemplateReviewTask[]
  approval_tasks: ContractTemplateApprovalTask[]
}

export interface ContractTemplateRetrieveByIdResponse extends ContractTemplateBaseResponse {
  state: ContractTemplateState
  name?: string
  description?: string
  template_type?: TemplateType
  created_by: string
  created_at: string
  updated_at: string
  /** The template data of the contract template */
  template_data: any
}

export interface ContractTemplateApproveResponse extends ContractTemplateBaseResponse {}

export interface ContractTemplateRejectResponse extends ContractTemplateBaseResponse {}
