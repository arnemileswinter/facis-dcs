package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"goa.design/goa/v3/security"

	pac "digital-contracting-service/gen/process_audit_and_compliance"
)

// DCS-FR-PACM-05: a Compliance Officer investigates non-compliance, so the
// report and case file exports admit the role.
func TestComplianceOfficerMayExportReportsAndCaseFiles(t *testing.T) {
	stop := errors.New("stop after authorization")
	var required []string
	auth := func(ctx context.Context, _ string, scheme *security.JWTScheme) (context.Context, error) {
		required = scheme.RequiredScopes
		return ctx, stop
	}
	endpoints := map[string]func() error{
		"audit_report": func() error {
			_, err := pac.NewAuditReportEndpoint(nil, auth)(context.Background(), &pac.AuditReportPayload{})
			return err
		},
		"case_file": func() error {
			_, err := pac.NewCaseFileEndpoint(nil, auth)(context.Background(), &pac.CaseFilePayload{})
			return err
		},
	}
	for name, call := range endpoints {
		required = nil
		if err := call(); !errors.Is(err, stop) {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Contains(required, "Compliance Officer") {
			t.Errorf("%s admits %v, not the Compliance Officer", name, required)
		}
	}
}
