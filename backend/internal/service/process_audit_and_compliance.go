package service

import (
	"context"
	processauditandcompliance "digital-contracting-service/gen/process_audit_and_compliance"
	"digital-contracting-service/internal/auth"

	"goa.design/clue/log"
)

type processAuditAndCompliancesrvc struct {
	auth.JWTAuthenticator
}

func NewProcessAuditAndCompliance(jwtAuth auth.JWTAuthenticator) processauditandcompliance.Service {
	return &processAuditAndCompliancesrvc{JWTAuthenticator: jwtAuth}
}

func (s *processAuditAndCompliancesrvc) Audit(ctx context.Context, p *processauditandcompliance.AuditPayload) (res string, err error) {
	log.Printf(ctx, "processAuditAndCompliance.audit")
	return
}

func (s *processAuditAndCompliancesrvc) AuditReport(ctx context.Context, p *processauditandcompliance.AuditReportPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.audit_report")
	return
}

func (s *processAuditAndCompliancesrvc) Monitor(ctx context.Context, p *processauditandcompliance.MonitorPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.monitor")
	return
}

func (s *processAuditAndCompliancesrvc) IncidentReport(ctx context.Context, p *processauditandcompliance.IncidentReportPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.incident_report")
	return
}
