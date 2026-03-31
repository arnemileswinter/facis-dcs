export interface ContractCreateRequest {
  did: string
}

export interface ContractUpdateRequest {
  did: string
  updated_at: string
  contract_version?: number
  name?: string
  description?: string
  /** The data of the contract */
  contract_data?: unknown
}

export interface ContractRetrieveRequest {}

export interface ContractRetrieveByIdRequest {
  did: string
}
