package command

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

const archiveSnapshotHashAlgorithm = "SHA-256"

// ArchiveSigningEvidence is one signature of an archived contract.
type ArchiveSigningEvidence struct {
	Signer, CredentialType, CeremonyID, Field, PDFCID, PDFHash string
	SignedAt                                                   time.Time
	CredentialHashes                                           map[string]string
}

// BuildArchiveEntry freezes the signed contract state for archive
// persistence (DCS-FR-CWE-20, DCS-FR-CSA-08): the archive entry is created
// once the signature workflow completes, with every signature, the credential
// hashes of each signature keyed by its ceremony, and the signatures a peer
// applied (peerSignatures, may be nil). The last signature names the final
// signed PDF.
func BuildArchiveEntry(contract *db.Contract, storedBy string, signatures []ArchiveSigningEvidence, peerSignatures json.RawMessage) (db.ContractArchiveEntry, error) {
	if contract == nil {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract is required")
	}
	if contract.State != contractstate.Signed.String() {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract %s must be signed before archive storage", contract.DID)
	}
	if len(signatures) == 0 {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract %s has no signature to archive", contract.DID)
	}

	snapshotJSON, err := buildContractSnapshot(contract)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	contentHash, err := HashArchiveSnapshot(snapshotJSON)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}

	entries := make([]map[string]any, 0, len(signatures))
	hashes := make(map[string]map[string]string, len(signatures))
	for _, signing := range signatures {
		entries = append(entries, map[string]any{
			"signer": signing.Signer, "credential_type": signing.CredentialType,
			"ceremony_id": signing.CeremonyID, "field": signing.Field, "signed_at": signing.SignedAt.UTC().Format(time.RFC3339Nano),
			"pdf_cid": signing.PDFCID, "pdf_hash": archivePDFHash(signing.PDFHash),
		})
		if len(signing.CredentialHashes) > 0 {
			hashes[signing.CeremonyID] = signing.CredentialHashes
		}
	}
	final := signatures[len(signatures)-1]
	metadata := map[string]any{
		"status": "SIGNED", "pdf_cid": final.PDFCID, "pdf_hash": archivePDFHash(final.PDFHash), "signatures": entries,
	}
	if len(peerSignatures) > 0 {
		metadata["peer_signatures"] = peerSignatures
	}
	signatureMetadata, err := datatype.NewJSON(metadata)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	credentialHashes, err := datatype.NewJSON(hashes)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	evidence, err := datatype.NewJSON(map[string]any{
		"source":                  "SIGNING_WORKFLOW_COMPLETION",
		"stored_by":               storedBy,
		"stored_state":            contractstate.Signed.String(),
		"snapshot_hash_algorithm": archiveSnapshotHashAlgorithm,
	})
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}

	storedAt := time.Now().UTC()
	return db.ContractArchiveEntry{
		DID:              contract.DID,
		ContractVersion:  contract.ContractVersion,
		StoredBy:         storedBy,
		StoredAt:         storedAt,
		RetentionUntil:   retentionUntil(contract.ExpDate, storedAt, conf.ArchiveRetentionYears()),
		ContractSnapshot: snapshotJSON,
		ContentHash:      contentHash,
		SignatureMeta:    &signatureMetadata,
		CredentialHashes: &credentialHashes,
		Evidence:         &evidence,
	}, nil
}

func archivePDFHash(hash string) string {
	return "sha256:" + strings.TrimPrefix(hash, "sha256:")
}

func buildContractSnapshot(contract *db.Contract) (datatype.JSON, error) {
	contractData := json.RawMessage(`{}`)
	if contract.ContractData != nil && contract.ContractData.IsNotNullValue() {
		contractData = json.RawMessage(*contract.ContractData)
	}

	snapshot := map[string]any{
		"did":               contract.DID,
		"contract_version":  contract.ContractVersion,
		"state":             contract.State,
		"name":              stringPtrValue(contract.Name),
		"description":       stringPtrValue(contract.Description),
		"created_by":        contract.CreatedBy,
		"created_at":        formatArchiveTime(&contract.CreatedAt),
		"updated_at":        formatArchiveTime(&contract.UpdatedAt),
		"start_date":        formatArchiveTime(contract.StartDate),
		"exp_date":          formatArchiveTime(contract.ExpDate),
		"exp_policy":        stringPtrValue(contract.ExpPolicy),
		"exp_notice_period": intPtrValue(contract.ExpNoticePeriod),
		"responsible":       contract.Responsible,
		"contract_data":     contractData,
	}

	return datatype.NewJSON(snapshot)
}

func HashArchiveSnapshot(snapshot datatype.JSON) (string, error) {
	canonicalSnapshot, err := CanonicalizeArchiveSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonicalSnapshot)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func CanonicalizeArchiveSnapshot(snapshot datatype.JSON) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(snapshot))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode archive snapshot JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode archive snapshot JSON: multiple JSON values")
		}
		return nil, fmt.Errorf("decode archive snapshot JSON: %w", err)
	}

	canonicalSnapshot, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonicalize archive snapshot JSON: %w", err)
	}
	return canonicalSnapshot, nil
}

func formatArchiveTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func intPtrValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// retentionUntil starts the retention period at the contract's expiry, or at
// archival for a contract without one. The archive trigger refuses deletion
// before this date. No period configured means no retention date.
func retentionUntil(expDate *time.Time, storedAt time.Time, years int) *time.Time {
	if years <= 0 {
		return nil
	}
	from := storedAt
	if expDate != nil {
		from = expDate.UTC()
	}
	until := from.AddDate(years, 0, 0)
	return &until
}
