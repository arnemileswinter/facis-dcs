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
)

// PartyReport is the compliance report of one contract segmented by party
// (DCS-FR-PACM-07). The segmentation by clause is GET /signature/clause-report.
// A party is COMPLIANT when its signature passes the checks named in
// PartyReportChecks, not because it is signed.
type PartyReport struct {
	ContractDID   string            `json:"contract_did"`
	ContractState string            `json:"contract_state"`
	GeneratedAt   time.Time         `json:"generated_at"`
	GeneratedBy   string            `json:"generated_by"`
	Checks        []string          `json:"checks"`
	Status        string            `json:"status"`
	Parties       []PartyCompliance `json:"parties"`
}

// PartyReportChecks are the checks a party's signature is judged by.
var PartyReportChecks = []string{
	"signature not revoked",
	"achieved signature level meets the required level",
	"signing certificate not revoked",
	"Power of Attorney authorizes the party",
}

// CredentialMetadata describes the credential behind one signature.
type CredentialMetadata struct {
	Party           string     `json:"party" db:"party"`
	SignerDID       string     `json:"signer_did" db:"signer_did"`
	SignatureStatus string     `json:"signature_status" db:"signature_status"`
	SignedAt        *time.Time `json:"signed_at,omitempty" db:"signed_at"`
	AchievedLevel   string     `json:"achieved_level" db:"achieved_level"`
	RequiredLevel   string     `json:"required_level" db:"required_level"`
	CertSubject     *string    `json:"signer_cert_subject,omitempty" db:"signer_cert_subject"`
	CertSerial      *string    `json:"signer_cert_serial,omitempty" db:"signer_cert_serial"`
	CertRevokedAt   *time.Time `json:"cert_revoked_at,omitempty" db:"cert_revoked_at"`
	PoAOrganization *string    `json:"poa_organization,omitempty" db:"poa_organization"`
	KbSdHash        *string    `json:"kb_sd_hash,omitempty" db:"kb_sd_hash"`
}

type PartyCompliance struct {
	Party           string              `json:"party"`
	Status          string              `json:"status"`
	Timestamp       *time.Time          `json:"timestamp,omitempty"`
	NonConformances []string            `json:"non_conformances"`
	Credential      *CredentialMetadata `json:"credential,omitempty"`
}

// ContractCredentials returns the credential metadata of every signature of
// the contract, oldest first.
func ContractCredentials(ctx context.Context, db sqlx.QueryerContext, did string) ([]CredentialMetadata, error) {
	var rows []struct {
		CredentialMetadata
		ContractData []byte `db:"contract_data"`
	}
	if err := sqlx.SelectContext(ctx, db, &rows, `
        SELECT COALESCE(s.field_name, '') AS party, s.signer_did, s.status::text AS signature_status,
               s.signed_at, s.credential_type AS achieved_level, s.cert_revoked_at,
               c.signer_cert_subject, c.signer_cert_serial, c.poa_organization, c.kb_sd_hash,
               k.contract_data
        FROM contract_signatures s
        JOIN contracts k ON k.did = s.contract_did
        LEFT JOIN signature_ceremonies c ON c.id = s.ceremony_id
        WHERE s.contract_did = $1 ORDER BY s.id`, did); err != nil {
		return nil, fmt.Errorf("could not read signatures: %w", err)
	}
	credentials := make([]CredentialMetadata, 0, len(rows))
	for _, row := range rows {
		row.RequiredLevel = validation.RequiredCredentialType(row.ContractData, row.Party)
		credentials = append(credentials, row.CredentialMetadata)
	}
	return credentials, nil
}

type PartyReportQry struct {
	DID         string
	GeneratedBy string
}

type PartyReporter struct {
	DB *sqlx.DB
}

// Handle returns the party report as JSON.
func (h *PartyReporter) Handle(ctx context.Context, query PartyReportQry) ([]byte, error) {
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
	credentials, err := ContractCredentials(ctx, h.DB, query.DID)
	if err != nil {
		return nil, err
	}
	report := buildPartyReport(query.DID, contract.State, validation.RequiredSignatureFields(contract.Data), credentials)
	report.GeneratedAt = time.Now().UTC()
	report.GeneratedBy = query.GeneratedBy
	return json.Marshal(report)
}

// buildPartyReport judges every declared signature field and every party a
// signature names. The newest signature of a party counts: a re-signature
// follows a revocation.
func buildPartyReport(did, state string, declared []string, credentials []CredentialMetadata) PartyReport {
	report := PartyReport{ContractDID: did, ContractState: state, Checks: PartyReportChecks, Status: StatusCompliant, Parties: []PartyCompliance{}}
	parties := append([]string{}, declared...)
	known := map[string]bool{}
	for _, party := range parties {
		known[party] = true
	}
	for _, credential := range credentials {
		if !known[credential.Party] {
			known[credential.Party] = true
			parties = append(parties, credential.Party)
		}
	}
	for _, party := range parties {
		entry := PartyCompliance{Party: party, Status: StatusPending, NonConformances: []string{}}
		for i := range credentials {
			if credentials[i].Party != party {
				continue
			}
			entry.Credential = &credentials[i]
			entry.Timestamp = credentials[i].SignedAt
			entry.NonConformances = nonConformances(credentials[i])
			entry.Status = StatusCompliant
			if len(entry.NonConformances) > 0 {
				entry.Status = StatusNonCompliant
			}
		}
		if entry.Status == StatusNonCompliant || (entry.Status == StatusPending && report.Status == StatusCompliant) {
			report.Status = entry.Status
		}
		report.Parties = append(report.Parties, entry)
	}
	return report
}

func nonConformances(credential CredentialMetadata) []string {
	found := []string{}
	if credential.SignatureStatus == "REVOKED" {
		found = append(found, "the signature is revoked")
	}
	if !validation.SignatureLevelSatisfies(credential.AchievedLevel, credential.RequiredLevel) {
		found = append(found, fmt.Sprintf("the signature achieved %s, the contract requires %s", credential.AchievedLevel, credential.RequiredLevel))
	}
	if credential.CertRevokedAt != nil {
		found = append(found, "the signing certificate is revoked")
	}
	if credential.PoAOrganization != nil && strings.TrimSpace(*credential.PoAOrganization) != credential.Party {
		found = append(found, fmt.Sprintf("the Power of Attorney authorizes %s, not this party", *credential.PoAOrganization))
	}
	return found
}
