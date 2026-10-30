package qry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/validation"
)

// ErrContractNotFound is returned when the contract of a report does not exist.
var ErrContractNotFound = errors.New("contract not found")

const (
	StatusCompliant    = "COMPLIANT"
	StatusNonCompliant = "NON_COMPLIANT"
	StatusPending      = "PENDING"
	StatusNotEvaluated = "NOT_EVALUATED"
)

// ComplianceReport is the compliance report of one contract segmented by
// party and by contract component (DCS-FR-PACM-07). Each segment has a
// compliance status, a timestamp, its non-conformances and the relevant
// credential metadata.
type ComplianceReport struct {
	ContractDID   string                `json:"contract_did"`
	ContractState string                `json:"contract_state"`
	GeneratedAt   time.Time             `json:"generated_at"`
	GeneratedBy   string                `json:"generated_by"`
	Status        string                `json:"status"`
	Parties       []PartyCompliance     `json:"parties"`
	Components    []ComponentCompliance `json:"components"`
}

type CredentialMetadata struct {
	SignerDID       string     `json:"signer_did"`
	SignatureStatus string     `json:"signature_status"`
	AchievedLevel   string     `json:"achieved_level"`
	RequiredLevel   string     `json:"required_level"`
	CertSubject     *string    `json:"signer_cert_subject,omitempty"`
	CertSerial      *string    `json:"signer_cert_serial,omitempty"`
	CertRevokedAt   *time.Time `json:"cert_revoked_at,omitempty"`
	PoAOrganization *string    `json:"poa_organization,omitempty"`
	KbSdHash        *string    `json:"kb_sd_hash,omitempty"`
}

type PartyCompliance struct {
	Party           string              `json:"party"`
	Status          string              `json:"status"`
	Timestamp       *time.Time          `json:"timestamp,omitempty"`
	NonConformances []string            `json:"non_conformances"`
	Credential      *CredentialMetadata `json:"credential,omitempty"`
}

type ComponentCompliance struct {
	Component       string               `json:"component"`
	Type            string               `json:"type,omitempty"`
	Status          string               `json:"status"`
	Timestamp       *time.Time           `json:"timestamp,omitempty"`
	NonConformances []string             `json:"non_conformances"`
	Credentials     []CredentialMetadata `json:"credentials"`
}

// reportSignature is one signature row with the ceremony it was made under.
type reportSignature struct {
	SignerDID       string     `db:"signer_did"`
	Field           string     `db:"field_name"`
	Achieved        string     `db:"credential_type"`
	Status          string     `db:"status"`
	SignedAt        *time.Time `db:"signed_at"`
	RevokedAt       *time.Time `db:"revoked_at"`
	CertRevokedAt   *time.Time `db:"cert_revoked_at"`
	CertSubject     *string    `db:"signer_cert_subject"`
	CertSerial      *string    `db:"signer_cert_serial"`
	PoAOrganization *string    `db:"poa_organization"`
	KbSdHash        *string    `db:"kb_sd_hash"`
}

// gateEvaluation is the local evaluation of the newest workflow gate run.
type gateEvaluation struct {
	Findings  []validation.PolicyFinding
	CheckedAt time.Time
}

type ComplianceReportQry struct {
	DID         string
	GeneratedBy string
}

type ComplianceReporter struct {
	DB *sqlx.DB
}

// Handle reads the contract, its signatures and its newest workflow gate run
// and returns the report as JSON.
func (h *ComplianceReporter) Handle(ctx context.Context, query ComplianceReportQry) ([]byte, error) {
	var contract struct {
		State string `db:"state"`
		Data  []byte `db:"contract_data"`
	}
	err := h.DB.GetContext(ctx, &contract, `SELECT state::text AS state, contract_data FROM contracts_effective WHERE did = $1`, query.DID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContractNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not read contract: %w", err)
	}
	var content map[string]any
	if err := json.Unmarshal(contract.Data, &content); err != nil {
		return nil, fmt.Errorf("could not decode contract: %w", err)
	}
	var signatures []reportSignature
	if err := h.DB.SelectContext(ctx, &signatures, `
        SELECT s.signer_did, COALESCE(s.field_name, '') AS field_name, s.credential_type, s.status::text AS status,
               s.signed_at, s.revoked_at, s.cert_revoked_at,
               c.signer_cert_subject, c.signer_cert_serial, c.poa_organization, c.kb_sd_hash
        FROM contract_signatures s LEFT JOIN signature_ceremonies c ON c.id = s.ceremony_id
        WHERE s.contract_did = $1 ORDER BY s.id`, query.DID); err != nil {
		return nil, fmt.Errorf("could not read signatures: %w", err)
	}
	var run *gateEvaluation
	var row struct {
		Local     []byte    `db:"local_evaluation"`
		CheckedAt time.Time `db:"checked_at"`
	}
	err = h.DB.GetContext(ctx, &row, `
        SELECT local_evaluation, COALESCE(completed_at, created_at) AS checked_at
        FROM pac_workflow_gate_runs WHERE contract_did = $1
        ORDER BY created_at DESC LIMIT 1`, query.DID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, fmt.Errorf("could not read workflow gate run: %w", err)
	default:
		var local struct {
			Findings []validation.PolicyFinding `json:"findings"`
		}
		if err := json.Unmarshal(row.Local, &local); err != nil {
			return nil, fmt.Errorf("could not decode workflow gate evaluation: %w", err)
		}
		run = &gateEvaluation{Findings: local.Findings, CheckedAt: row.CheckedAt}
	}
	report := buildComplianceReport(query.DID, contract.State, content, signatures, run)
	report.GeneratedAt = time.Now().UTC()
	report.GeneratedBy = query.GeneratedBy
	return json.Marshal(report)
}

func buildComplianceReport(did, state string, content map[string]any, signatures []reportSignature, run *gateEvaluation) ComplianceReport {
	raw, _ := json.Marshal(content)
	report := ComplianceReport{ContractDID: did, ContractState: state, Parties: []PartyCompliance{}, Components: []ComponentCompliance{}}

	// Parties: every declared signature field, and every field a signature names.
	parties := validation.RequiredSignatureFields(raw)
	declared := map[string]bool{}
	for _, party := range parties {
		declared[party] = true
	}
	for _, sig := range signatures {
		if !declared[sig.Field] {
			declared[sig.Field] = true
			parties = append(parties, sig.Field)
		}
	}
	var credentials []CredentialMetadata
	for _, party := range parties {
		entry := PartyCompliance{Party: party, Status: StatusPending, NonConformances: []string{}}
		// The newest signature of the party counts: a re-signature follows a revocation.
		for _, sig := range signatures {
			if sig.Field != party {
				continue
			}
			credential := credentialOf(sig, validation.RequiredCredentialType(raw, party))
			entry.Credential = &credential
			entry.Timestamp = sig.SignedAt
			entry.NonConformances = signatureNonConformances(party, sig, credential.RequiredLevel)
			entry.Status = StatusCompliant
			if len(entry.NonConformances) > 0 {
				entry.Status = StatusNonCompliant
			}
		}
		if entry.Credential != nil && entry.Credential.SignatureStatus == "SIGNED" {
			credentials = append(credentials, *entry.Credential)
		}
		report.Parties = append(report.Parties, entry)
	}
	if credentials == nil {
		credentials = []CredentialMetadata{}
	}

	// Components: the domain objects of dcs:contractData, and the contract as
	// a whole for findings that name no field of a component.
	owner := map[string]string{}
	components := []ComponentCompliance{{Component: did, Type: "dcs:Contract"}}
	objects, _ := content["dcs:contractData"].([]any)
	for _, object := range objects {
		node, ok := object.(map[string]any)
		id, _ := node["@id"].(string)
		if !ok || id == "" {
			continue
		}
		kind, _ := node["@type"].(string)
		components = append(components, ComponentCompliance{Component: id, Type: kind})
		collectFieldRefs(node, func(ref string) {
			if ref != id {
				owner[ref] = id
			}
		})
	}
	nonConformances := map[string][]string{}
	if run != nil {
		for _, finding := range run.Findings {
			if !isNonConformance(finding.Severity) {
				continue
			}
			component := did
			if id, ok := owner[finding.FieldIri]; ok {
				component = id
			}
			nonConformances[component] = append(nonConformances[component], findingText(finding))
		}
	}
	for _, component := range components {
		component.Credentials = credentials
		component.NonConformances = nonConformances[component.Component]
		if component.NonConformances == nil {
			component.NonConformances = []string{}
		}
		component.Status = StatusNotEvaluated
		if run != nil {
			checkedAt := run.CheckedAt
			component.Timestamp = &checkedAt
			component.Status = StatusCompliant
			if len(component.NonConformances) > 0 {
				component.Status = StatusNonCompliant
			}
		}
		report.Components = append(report.Components, component)
	}

	report.Status = StatusCompliant
	for _, party := range report.Parties {
		report.Status = worse(report.Status, party.Status)
	}
	for _, component := range report.Components {
		report.Status = worse(report.Status, component.Status)
	}
	return report
}

func credentialOf(sig reportSignature, required string) CredentialMetadata {
	return CredentialMetadata{
		SignerDID: sig.SignerDID, SignatureStatus: sig.Status, AchievedLevel: sig.Achieved, RequiredLevel: required,
		CertSubject: sig.CertSubject, CertSerial: sig.CertSerial, CertRevokedAt: sig.CertRevokedAt,
		PoAOrganization: sig.PoAOrganization, KbSdHash: sig.KbSdHash,
	}
}

func signatureNonConformances(party string, sig reportSignature, required string) []string {
	found := []string{}
	if sig.Status == "REVOKED" {
		found = append(found, "the signature is revoked")
	}
	if !validation.SignatureLevelSatisfies(sig.Achieved, required) {
		found = append(found, fmt.Sprintf("the signature achieved %s, the contract requires %s", sig.Achieved, required))
	}
	if sig.CertRevokedAt != nil {
		found = append(found, "the signing certificate is revoked")
	}
	if sig.PoAOrganization != nil && strings.TrimSpace(*sig.PoAOrganization) != party {
		found = append(found, fmt.Sprintf("the Power of Attorney authorizes %s, not this party", *sig.PoAOrganization))
	}
	return found
}


func isNonConformance(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case validation.SeverityError, validation.SeverityWarning, "critical", "blocking", "violation", "warn", "review":
		return true
	}
	return false
}

func findingText(finding validation.PolicyFinding) string {
	if finding.RuleID == "" {
		return finding.Message
	}
	return finding.RuleID + ": " + finding.Message
}

// collectFieldRefs visits every {"@id": …} reference inside a domain object.
func collectFieldRefs(value any, visit func(string)) {
	switch typed := value.(type) {
	case map[string]any:
		if id, ok := typed["@id"].(string); ok && len(typed) == 1 {
			visit(id)
			return
		}
		for key, child := range typed {
			if key != "@id" {
				collectFieldRefs(child, visit)
			}
		}
	case []any:
		for _, child := range typed {
			collectFieldRefs(child, visit)
		}
	}
}

// worse orders the statuses NON_COMPLIANT > NOT_EVALUATED > PENDING > COMPLIANT.
func worse(a, b string) string {
	rank := map[string]int{StatusCompliant: 0, StatusPending: 1, StatusNotEvaluated: 2, StatusNonCompliant: 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
