import type { TemplateType } from '@/types/template-type'
import type { ActionFlag } from '../../types/action-flag'
import type { ContractTemplateState } from '@/types/contract-template-state'
import type { ContractTemplateData } from '../contract-template'

interface ContractTemplateBaseRequest {
  did: string
  document_number: number
  version: number
}

export interface ContractTemplateCreateRequest {
  name?: string
  description?: string
  template_type?: TemplateType
  /** The template data of the contract template */
  template_data?: ContractTemplateData
}

export interface ContractTemplateSubmitRequest extends ContractTemplateBaseRequest {
  updated_at: string
  forward_to?: ActionFlag
  comments?: string[]
}

export interface ContractTemplateUpdateRequest extends ContractTemplateBaseRequest {
  updated_at: string
  name?: string
  description?: string
  /** The template data of the contract template */
  template_data?: ContractTemplateData
}

export interface ContractTemplateSearchRequest {
  did?: string
  document_number?: number
  version?: number
  template_type?: TemplateType
  state?: ContractTemplateState
  name?: string
  description?: string
  filter?: string
}

export interface ContractTemplateRetrieveRequest {}

export interface ContractTemplateRetrieveByIdRequest extends ContractTemplateBaseRequest {}

export interface ContractTemplateApproveRequest extends ContractTemplateBaseRequest {
  updated_at: string
  decision_notes?: string[]
}

export interface ContractTemplateRejectRequest extends ContractTemplateBaseRequest {
  updated_at: string
  /** Reason for rejecting the contract template */
  reason: string
}
