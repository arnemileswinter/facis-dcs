import type { PACIncidentReportRequest } from '@/models/requests/pac-non-compliance-request'
import type { PACMonitorResponse } from '@/models/responses/pac-non-compliance-response'

export interface PACNonComplianceService {
  monitor: () => Promise<PACMonitorResponse>
  reportIncident: (request: PACIncidentReportRequest) => Promise<string | undefined>
  exportCaseFile: (caseId: string, justification: string) => Promise<Blob>
  exportComplianceReport: (did: string) => Promise<Blob>
}
