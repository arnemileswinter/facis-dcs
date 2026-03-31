import type { Contract } from '@/models/contract/contract'
import type {
  ContractCreateRequest,
  ContractRetrieveRequest,
  ContractRetrieveByIdRequest,
  ContractUpdateRequest,
} from '@/models/requests/contract-requests'
import type {
  ContractCreateResponse,
  ContractRetrieveResponse,
  ContractUpdateResponse,
} from '@/models/responses/contract-response'

export interface ContractWorkflowService {
  create: (request: ContractCreateRequest) => Promise<ContractCreateResponse>
  update: (request: ContractUpdateRequest) => Promise<ContractUpdateResponse>
  retrieve: (request?: ContractRetrieveRequest) => Promise<ContractRetrieveResponse>
  retrieveById: (request: ContractRetrieveByIdRequest) => Promise<Contract | null>
}
