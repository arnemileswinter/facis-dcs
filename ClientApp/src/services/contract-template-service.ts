import http from '@/api/http'
import type { ContractTemplate } from '@/models/contract-template'
import type {
  ContractTemplateApproveRequest,
  ContractTemplateCreateRequest,
  ContractTemplateRejectRequest,
  ContractTemplateRetrieveByIdRequest,
  ContractTemplateRetrieveRequest,
  ContractTemplateSearchRequest,
  ContractTemplateSubmitRequest,
  ContractTemplateUpdateRequest,
} from '@/models/requests/template-request'
import type {
  ContractTemplateApproveResponse,
  ContractTemplateCreateResponse,
  ContractTemplateRejectResponse,
  ContractTemplateRetrieveByIdResponse,
  ContractTemplateRetrieveResponse,
  ContractTemplateSearchResponse,
  ContractTemplateSubmitResponse,
  ContractTemplateUpdateResponse,
} from '@/models/responses/template-response'

export const ContractTemplateService = {
  async create(request: ContractTemplateCreateRequest) {
    return http.post<ContractTemplateCreateResponse>('/template/create', request).then((res) => res.data)
  },

  async submit(request: ContractTemplateSubmitRequest) {
    return http.post<ContractTemplateSubmitResponse>('/template/submit', request).then((res) => res.data)
  },

  async update(request: ContractTemplateUpdateRequest) {
    return http.put<ContractTemplateUpdateResponse>('/template/update', request).then((res) => res.data)
  },

  async search(request: ContractTemplateSearchRequest) {
    return http
      .get<ContractTemplateSearchResponse>('/template/search', { params: request })
      .then((res) => res.data.search_results)
      .catch((err) => {
        console.error('Search Error:', err)
        return []
      })
  },

  async retrieve(_request?: ContractTemplateRetrieveRequest) {
    return http
      .get<ContractTemplateRetrieveResponse>('/template/retrieve')
      .then((res) => {
        return Array.isArray(res.data.contract_templates) ? res.data.contract_templates : []
      })
      .catch((err) => {
        console.error('Retrieve Error:', err)
        return []
      })
  },

  async retrieveById(request: ContractTemplateRetrieveByIdRequest): Promise<ContractTemplate | null> {
    const queryParams = { document_number: request.document_number, version: request.version }
    return http
      .get<ContractTemplateRetrieveByIdResponse>(`/template/retrieve/${request.did}`, { params: queryParams })
      .then((res) => {
        console.log(res.status)
        return { ...res.data }
      })
      .catch((err) => {
        console.error('Retrieve ID Error:', err.message)
        return null
      })
  },

  async approve(request: ContractTemplateApproveRequest) {
    return http.post<ContractTemplateApproveResponse>('/template/approve', request).then((res) => res.data)
  },

  async reject(request: ContractTemplateRejectRequest) {
    return http.post<ContractTemplateRejectResponse>('/template/reject', request).then((res) => res.data)
  },
}
