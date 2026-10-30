package workflowgate

import (
	"strings"
	"testing"

	"digital-contracting-service/internal/base/validation"
)

func contractRequiring(level string) map[string]any {
	return map[string]any{
		"dcs:signatureFields": []any{
			map[string]any{"dcs:signatoryName": "did:web:org-a", "dcs:requiredCredentialType": level},
		},
		"dcs:contractFields": []any{map[string]any{"@id": "#customer", "dcs:value": "Org A GmbH"}},
	}
}

func findingFor(t *testing.T, findings []validation.PolicyFinding, rule string) validation.PolicyFinding {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == rule && finding.Severity != validation.SeveritySatisfied {
			return finding
		}
	}
	for _, finding := range findings {
		if finding.RuleID == rule {
			return finding
		}
	}
	t.Fatalf("no finding for rule %s in %+v", rule, findings)
	return validation.PolicyFinding{}
}

func TestRegulatoryRulePackPassesACompliantContract(t *testing.T) {
	findings := regulatoryFindings(contractRequiring("AES"), regulatoryFacts{
		Signatures: []signedField{{Field: "did:web:org-a", Signer: "did:key:jane", Achieved: "AES"}},
		PIDValues:  []string{"SMITH", "JANE SMITH"},
	})
	if got := resultFromLocal(findings); got != "PASSED" {
		t.Fatalf("result = %s, want PASSED: %+v", got, findings)
	}
	for _, rule := range []string{RuleEIDASLevel, RuleQualifiedTSA, RuleGDPRRetention, RuleNoPIDInPDF} {
		findingFor(t, findings, rule)
	}
	if got := findingFor(t, findings, RuleGDPRRetention).Severity; got != validation.SeverityInfo {
		t.Fatalf("missing retention period severity = %s, want info", got)
	}
}

// The eIDAS level gate (ADR-37): a signature below the level its field
// requires blocks execution and the refusal names the rule.
func TestEIDASLevelGateBlocksASignatureBelowTheRequiredLevel(t *testing.T) {
	facts := regulatoryFacts{
		Signatures:   []signedField{{Field: "did:web:org-a", Signer: "did:key:jane", Achieved: "AES"}},
		TSAQualified: true,
	}
	findings := regulatoryFindings(contractRequiring("QES"), facts)
	if got := findingFor(t, findings, RuleEIDASLevel).Severity; got != validation.SeverityError {
		t.Fatalf("eIDAS level severity = %s, want error", got)
	}
	if got := resultFromLocal(findings); got != "BLOCKED" {
		t.Fatalf("result = %s, want BLOCKED", got)
	}
	blocking := (&LocalEvaluationBlockedError{Findings: blockingFindings(findings)}).Error()
	if !strings.Contains(blocking, RuleEIDASLevel) {
		t.Fatalf("refusal %q does not name %s", blocking, RuleEIDASLevel)
	}

	facts.Signatures[0].Achieved = "QES"
	if got := resultFromLocal(regulatoryFindings(contractRequiring("QES"), facts)); got != "PASSED" {
		t.Fatalf("QES signature on a QES field: result = %s, want PASSED", got)
	}
}

func TestQualifiedTimestampRuleBlocksQESWithoutAQualifiedTSA(t *testing.T) {
	findings := regulatoryFindings(contractRequiring("QES"), regulatoryFacts{
		Signatures: []signedField{{Field: "did:web:org-a", Signer: "did:key:jane", Achieved: "QES"}},
	})
	if got := findingFor(t, findings, RuleQualifiedTSA).Severity; got != validation.SeverityError {
		t.Fatalf("qualified TSA severity = %s, want error", got)
	}
}

func TestPIDInTheContractDocumentIsFlaggedForReview(t *testing.T) {
	content := contractRequiring("AES")
	content["dcs:contractFields"] = []any{map[string]any{"@id": "#contact", "dcs:value": "Jane Smith"}}
	findings := regulatoryFindings(content, regulatoryFacts{
		Signatures: []signedField{{Field: "did:web:org-a", Signer: "did:key:jane", Achieved: "AES"}},
		PIDValues:  pidValues([]byte(`{"given_name":"JANE","family_name":"SMITH"}`)),
	})
	if got := findingFor(t, findings, RuleNoPIDInPDF).Severity; got != validation.SeverityWarning {
		t.Fatalf("PID severity = %s, want warning", got)
	}
	if got := resultFromLocal(findings); got != "REVIEW" {
		t.Fatalf("result = %s, want REVIEW", got)
	}
}

func TestHierarchyRuleNeedsTheParentInForce(t *testing.T) {
	content := contractRequiring("AES")
	content["dcs:parentContract"] = map[string]any{"@id": "did:web:dcs:contract:frame"}
	if got := parentContractDID(content); got != "did:web:dcs:contract:frame" {
		t.Fatalf("parent = %q", got)
	}
	for state, want := range map[string]string{"": "REVIEW", "DRAFT": "BLOCKED", "TERMINATED": "BLOCKED", "SIGNED": "PASSED", "ACTIVE": "PASSED"} {
		findings := regulatoryFindings(content, regulatoryFacts{ParentDID: "did:web:dcs:contract:frame", ParentState: state})
		if got := resultFromLocal(findings); got != want {
			t.Errorf("parent state %q: result = %s, want %s", state, got, want)
		}
	}
}
