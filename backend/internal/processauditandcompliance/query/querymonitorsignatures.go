package qry

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	event2 "digital-contracting-service/internal/processauditandcompliance/event"
)

// RiskTypeLateSignature flags a contract whose start date passed before it
// was fully signed (DCS-FR-PACM-05).
const RiskTypeLateSignature = "LATE_SIGNATURE"

// RiskTypeInvalidCredential flags a signature that relies on a certificate
// the CRL check found revoked (DCS-FR-PACM-05).
const RiskTypeInvalidCredential = "INVALID_CREDENTIAL"

// lateSignature is a contract that waits for signatures after its start date.
// When the last signature arrives the risk resolves; the register keeps it.
type lateSignature struct {
	DID       string    `db:"did"`
	StartDate time.Time `db:"start_date"`
}

type revokedCredential struct {
	DID       string    `db:"did"`
	Signer    string    `db:"signer_did"`
	Field     string    `db:"field_name"`
	RevokedAt time.Time `db:"cert_revoked_at"`
}

// signatureRisks reads the contracts that wait for signatures after their
// start date and the signatures on revoked certificates. The details name
// only persisted facts, so a risk hashes the same on every sweep.
func signatureRisks(ctx context.Context, tx *sqlx.Tx, checkedAt time.Time) ([]event2.ComplianceRisk, error) {
	var late []lateSignature
	if err := tx.SelectContext(ctx, &late, `
        SELECT did, start_date FROM contracts_effective
        WHERE state = 'APPROVED' AND start_date < $1
        ORDER BY did`, checkedAt); err != nil {
		return nil, fmt.Errorf("could not read late signatures: %w", err)
	}
	var revoked []revokedCredential
	if err := tx.SelectContext(ctx, &revoked, `
        SELECT contract_did AS did, signer_did, COALESCE(field_name, '') AS field_name, cert_revoked_at
        FROM contract_signatures
        WHERE status = 'SIGNED' AND cert_revoked_at IS NOT NULL
        ORDER BY id`); err != nil {
		return nil, fmt.Errorf("could not read signatures on revoked certificates: %w", err)
	}
	return signatureRisksFrom(late, revoked, checkedAt), nil
}

func signatureRisksFrom(late []lateSignature, revoked []revokedCredential, checkedAt time.Time) []event2.ComplianceRisk {
	risks := make([]event2.ComplianceRisk, 0, len(late)+len(revoked))
	for _, contract := range late {
		risks = append(risks, event2.ComplianceRisk{
			DID:      contract.DID,
			RiskType: RiskTypeLateSignature,
			Detail: fmt.Sprintf("contract is not fully signed although its start date %s has passed",
				contract.StartDate.UTC().Format(time.RFC3339)),
			DetectedAt: checkedAt,
		})
	}
	for _, sig := range revoked {
		risks = append(risks, event2.ComplianceRisk{
			DID:      sig.DID,
			RiskType: RiskTypeInvalidCredential,
			Detail: fmt.Sprintf("signature of %s for %q relies on a certificate revoked at %s",
				sig.Signer, sig.Field, sig.RevokedAt.UTC().Format(time.RFC3339)),
			DetectedAt: checkedAt,
		})
	}
	return risks
}
