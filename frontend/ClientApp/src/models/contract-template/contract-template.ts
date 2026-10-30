import type { DcsTemplateData } from '../dcs-jsonld'
import type { ContractTemplateResponsible } from './contract-template-responsible'
import type { ContractTemplateState } from '@/types/contract-template-state'
import type { PartyType } from '@/types/party-type'
import type { TemplateType } from '@/types/template-type'

export const templateComplianceStatuses = ['UNASSESSED', 'COMPLIANT', 'NON_COMPLIANT'] as const
export type TemplateComplianceStatus = (typeof templateComplianceStatuses)[number]

/** DCS-FR-TR-10: the searchable template metadata. */
export interface TemplateSearchableMetadata {
  jurisdiction?: string
  industry?: string
  compliance_status?: TemplateComplianceStatus
}

export interface ContractTemplate extends TemplateSearchableMetadata {
  did: string
  created_by: string
  created_at: string
  version: number
  party_type: PartyType
  template_type: TemplateType
  state: ContractTemplateState
  name?: string
  description?: string
  template_data?: DcsTemplateData
  updated_at: string
  responsible?: ContractTemplateResponsible
  outdated?: boolean
  latest_did?: string
}

export type PartialContractTemplate = ContractTemplate

export type ContractTemplateData = DcsTemplateData
