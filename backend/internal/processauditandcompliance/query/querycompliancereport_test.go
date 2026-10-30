package qry

import (
	"testing"
	"time"

	"digital-contracting-service/internal/base/validation"
)

func ptr[T any](v T) *T { return &v }

// DCS-FR-PACM-07: the report is segmented by party and by contract component,
// and every segment carries status, timestamp, non-conformances and the
// credential metadata.
func TestComplianceReportSegmentsByPartyAndComponent(t *testing.T) {
	const did = "did:web:dcs:contract:c1"
	content := map[string]any{
		"dcs:signatureFields": []any{
			map[string]any{"dcs:signatoryName": "did:web:org-a", "dcs:requiredCredentialType": "QES"},
			map[string]any{"dcs:signatoryName": "did:web:org-b"},
		},
		"dcs:contractData": []any{
			map[string]any{"@id": did + "#payment", "@type": "dcs:PaymentTerms", "dcs:fee": map[string]any{"@id": did + "#field-fee"}},
			map[string]any{"@id": did + "#sla", "@type": "dcs:ServiceLevel", "dcs:availability": map[string]any{"@id": did + "#field-availability"}},
		},
	}
	signedAt := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	signatures := []reportSignature{{
		SignerDID: "did:key:jane", Field: "did:web:org-a", Achieved: "AES", Status: "SIGNED", SignedAt: &signedAt,
		CertSerial: ptr("0A1B"), CertSubject: ptr("CN=Jane Smith"), PoAOrganization: ptr("did:web:org-a"),
	}}
	checkedAt := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	run := &gateEvaluation{CheckedAt: checkedAt, Findings: []validation.PolicyFinding{
		{RuleID: "sla.availability", Severity: validation.SeverityError, Message: "availability below 99.5", FieldIri: did + "#field-availability"},
		{RuleID: "payment.fee", Severity: validation.SeveritySatisfied, FieldIri: did + "#field-fee"},
		{RuleID: "regulatory.gdpr.no-pid-in-pdf", Severity: validation.SeverityWarning, Message: "PID found"},
	}}

	report := buildComplianceReport(did, "SIGNED", content, signatures, run)

	if len(report.Parties) != 2 {
		t.Fatalf("parties = %+v", report.Parties)
	}
	a, b := report.Parties[0], report.Parties[1]
	if a.Party != "did:web:org-a" || a.Status != StatusNonCompliant || a.Timestamp == nil || !a.Timestamp.Equal(signedAt) {
		t.Errorf("party a = %+v", a)
	}
	if a.Credential == nil || a.Credential.AchievedLevel != "AES" || a.Credential.RequiredLevel != "QES" || *a.Credential.CertSerial != "0A1B" {
		t.Errorf("party a credential = %+v", a.Credential)
	}
	if len(a.NonConformances) != 1 {
		t.Errorf("party a non-conformances = %v", a.NonConformances)
	}
	if b.Party != "did:web:org-b" || b.Status != StatusPending || b.Credential != nil {
		t.Errorf("party b = %+v", b)
	}

	byID := map[string]ComponentCompliance{}
	for _, component := range report.Components {
		byID[component.Component] = component
		if component.Timestamp == nil || !component.Timestamp.Equal(checkedAt) {
			t.Errorf("component %s timestamp = %v", component.Component, component.Timestamp)
		}
		if len(component.Credentials) != 1 || component.Credentials[0].SignerDID != "did:key:jane" {
			t.Errorf("component %s credentials = %+v", component.Component, component.Credentials)
		}
	}
	if got := byID[did+"#sla"]; got.Status != StatusNonCompliant || len(got.NonConformances) != 1 {
		t.Errorf("sla component = %+v", got)
	}
	if got := byID[did+"#payment"]; got.Status != StatusCompliant || got.Type != "dcs:PaymentTerms" {
		t.Errorf("payment component = %+v", got)
	}
	if got := byID[did]; got.Status != StatusNonCompliant || len(got.NonConformances) != 1 {
		t.Errorf("contract component = %+v", got)
	}
	if report.Status != StatusNonCompliant {
		t.Errorf("report status = %s", report.Status)
	}

	if unevaluated := buildComplianceReport(did, "DRAFT", content, nil, nil); unevaluated.Status != StatusNotEvaluated {
		t.Errorf("report without gate run status = %s, want %s", unevaluated.Status, StatusNotEvaluated)
	}
}
