import type { ContractState } from '@/types/contract-state'
import type { ContractNegotiation } from './contract-negotiation'
import type { ContractData } from '../contract-data'

export interface Contract {
  did: string
  contract_version?: number
  state: ContractState
  name?: string
  description?: string
  created_at: string
  updated_at: string
  contract_data?: ContractData
  negotiations?: ContractNegotiation[]
}
