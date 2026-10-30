import http from '@/api/http'
import type { PACIncidentReportRequest } from '@/models/requests/pac-non-compliance-request'
import type { PACMonitorResponse } from '@/models/responses/pac-non-compliance-response'
import type { PACNonComplianceService } from '@/models/services/pac-non-compliance-service'

export const pacNonComplianceService: PACNonComplianceService = {
  async monitor() {
    return http.get<PACMonitorResponse>('/pac/monitor').then((res) => res.data)
  },

  async reportIncident(request: PACIncidentReportRequest) {
    return http.post<{ case_id?: string }>('/pac/report', request).then((res) => res.data?.case_id)
  },

  async exportCaseFile(caseId: string, justification: string) {
    return http
      .get<Blob>(`/pac/case/${encodeURIComponent(caseId)}`, { params: { justification }, responseType: 'blob' })
      .then((res) => res.data)
  },

  async exportComplianceReport(did: string) {
    return http.get<Blob>('/pac/compliance-report', { params: { did }, responseType: 'blob' }).then((res) => res.data)
  },
}
