import http from '@/api/http'
import type {
  ContractCreateRequest,
  ContractRetrieveByIdRequest,
  ContractRetrieveRequest,
  ContractUpdateRequest,
} from '@/models/requests/contract-requests'
import type {
  ContractRetrieveByIdResponse,
  ContractRetrieveResponse,
  ContractUpdateResponse,
} from '@/models/responses/contract-response'
import type { ContractWorkflowService } from '@/models/services/contract-workflow-service'

export const contractWorkflowService: ContractWorkflowService = {
  async create(request: ContractCreateRequest) {
    return http.post('/contract/create', request).then((res) => res.data)
  },

  async update(request: ContractUpdateRequest) {
    return http.put<ContractUpdateResponse>('/contract/update', request).then((res) => res.data)
  },

  async retrieve(_request?: ContractRetrieveRequest) {
    return http
      .get<ContractRetrieveResponse>('/contract/retrieve')
      .then((res) => res.data)
      .catch((err) => {
        console.error('Retrieve Error:', err)
        return { contracts: [], review_tasks: [], approval_tasks: [] } as ContractRetrieveResponse
      })
  },

  async retrieveById(request: ContractRetrieveByIdRequest) {
    return http
      .get<ContractRetrieveByIdResponse>(`/contract/${request.did}`)
      .then((res) => ({ ...res.data }))
      .catch((err) => {
        console.error('Retrieve ID Error:', err)
        return null
      })
  },
}
