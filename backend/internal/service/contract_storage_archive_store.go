package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	contractstoragearchive "digital-contracting-service/gen/contract_storage_archive"
	"digital-contracting-service/internal/base/artifactstore"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	baseevent "digital-contracting-service/internal/base/event"
	cwecommand "digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"digital-contracting-service/internal/middleware"
)

// archiveArtifacts is the encrypting IPFS store: content is encrypted under
// the contract's key, so the erasure of the contract erases it too.
type archiveArtifacts interface {
	Put(ctx context.Context, scope artifactstore.Scope, plaintext []byte) (string, error)
	Get(ctx context.Context, scope artifactstore.Scope, cid string) ([]byte, error)
}

// Store archives a contract or stores an evidence document for an archived
// contract (DCS-IR-CSA-02, DCS-FR-CSA-25).
func (s *contractStorageArchivesrvc) Store(ctx context.Context, p *contractstoragearchive.StorePayload) (res *contractstoragearchive.ArchiveStoreResponse, err error) {
	defer func() { s.auditFailure(ctx, "store", p.Did, err) }()

	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	if s.Artifacts == nil {
		return nil, contractstoragearchive.MakeInternalError(errors.New("artifact store is not configured"))
	}
	switch p.Kind {
	case "contract":
		return s.storeContract(ctx, p.Did)
	case "evidence":
		return s.storeEvidence(ctx, p)
	case "section":
		return s.storeSection(ctx, p)
	}
	return nil, contractstoragearchive.MakeBadRequest(fmt.Errorf("unknown kind %q", p.Kind))
}

func (s *contractStorageArchivesrvc) storeContract(ctx context.Context, did string) (*contractstoragearchive.ArchiveStoreResponse, error) {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	var known bool
	if err := tx.GetContext(ctx, &known, `SELECT EXISTS (SELECT 1 FROM contracts WHERE did = $1)`, did); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if !known {
		return nil, contractstoragearchive.MakeBadRequest(fmt.Errorf("no contract with DID %q", did))
	}
	contract, err := s.CRepo.ReadDataByDID(ctx, tx, did)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	storedBy := middleware.GetParticipantID(ctx)
	history, err := s.CRepo.ReadHistoryByDID(ctx, tx, did)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	entry, err := cwecommand.BuildManualArchiveEntry(contract, history, storedBy)
	if err != nil {
		return nil, contractstoragearchive.MakeBadRequest(err)
	}
	var archived bool
	if err := tx.GetContext(ctx, &archived,
		`SELECT EXISTS (SELECT 1 FROM contract_archive_entries WHERE did = $1 AND contract_version = $2)`,
		did, entry.ContractVersion); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if archived {
		return nil, contractstoragearchive.MakeBadRequest(
			fmt.Errorf("version %d of contract %q is already archived", entry.ContractVersion, did))
	}

	entry.SnapshotCID, err = s.Artifacts.Put(ctx, artifactstore.ContractScope(did), entry.ContractSnapshot)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(fmt.Errorf("store archive snapshot: %w", err))
	}
	notaryReceipt, tsaReceipt, err := cwecommand.NotarizeArchiveEntry(ctx, s.ArchiveNotary, s.ArchiveTSA, &entry)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := s.CRepo.StoreArchiveEntry(ctx, tx, entry); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	evt := contractevents.StoreArchivedEvent{
		DID:             did,
		ContractVersion: entry.ContractVersion,
		StoredBy:        storedBy,
		ContentHash:     entry.ContentHash,
		SnapshotCID:     entry.SnapshotCID,
		ArchiveStatus:   "STORED",
		NotaryReceipt:   notaryReceipt,
		TSAReceipt:      tsaReceipt,
		EvidenceSummary: contractevents.ArchiveEvidenceSummary{
			SnapshotHashAlgorithm: "SHA-256",
			SignatureStatus:       "NOT_RECORDED",
			CredentialHashStatus:  "NOT_RECORDED",
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := baseevent.Create(ctx, tx, evt, componenttype.ContractStorageArchive); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	version := entry.ContractVersion
	return &contractstoragearchive.ArchiveStoreResponse{Did: did, Kind: "contract", ContractVersion: &version, ContentHash: entry.ContentHash}, nil
}

func (s *contractStorageArchivesrvc) storeEvidence(ctx context.Context, p *contractstoragearchive.StorePayload) (*contractstoragearchive.ArchiveStoreResponse, error) {
	name, mediaType := strings.TrimSpace(stringValue(p.Name)), strings.TrimSpace(stringValue(p.MediaType))
	if name == "" || mediaType == "" || len(p.Content) == 0 {
		return nil, contractstoragearchive.MakeBadRequest(errors.New("evidence needs a name, a media type and content"))
	}

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	var archived bool
	if err := tx.GetContext(ctx, &archived,
		`SELECT EXISTS (SELECT 1 FROM contract_archive_entries WHERE did = $1 AND archive_status <> 'DELETED')`, p.Did); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if !archived {
		return nil, contractstoragearchive.MakeBadRequest(fmt.Errorf("no live archive entry for DID %q", p.Did))
	}

	sum := sha256.Sum256(p.Content)
	contentHash := "sha256:" + hex.EncodeToString(sum[:])
	cid, err := s.Artifacts.Put(ctx, artifactstore.ContractScope(p.Did), p.Content)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(fmt.Errorf("store evidence content: %w", err))
	}
	storedBy := middleware.GetParticipantID(ctx)
	var id string
	if err := tx.GetContext(ctx, &id, `
		INSERT INTO contract_archive_evidence (did, name, media_type, description, content_hash, content_cid, size_bytes, stored_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		p.Did, name, mediaType, p.Description, contentHash, cid, len(p.Content), storedBy); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	evt := contractevents.StoreArchiveEvidenceEvent{
		DID:         p.Did,
		EvidenceID:  id,
		StoredBy:    storedBy,
		Name:        name,
		MediaType:   mediaType,
		ContentHash: contentHash,
		OccurredAt:  time.Now().UTC(),
	}
	if err := baseevent.Create(ctx, tx, evt, componenttype.ContractStorageArchive); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return &contractstoragearchive.ArchiveStoreResponse{Did: p.Did, Kind: "evidence", EvidenceID: &id, ContentHash: contentHash}, nil
}

type archiveEvidenceRow struct {
	ID          string    `db:"id"`
	DID         string    `db:"did"`
	Name        string    `db:"name"`
	MediaType   string    `db:"media_type"`
	Description *string   `db:"description"`
	ContentHash string    `db:"content_hash"`
	ContentCID  string    `db:"content_cid"`
	SizeBytes   int64     `db:"size_bytes"`
	StoredBy    string    `db:"stored_by"`
	StoredAt    time.Time `db:"stored_at"`
}

// Evidence lists the evidence documents of an archived contract with their
// decrypted content. Content of an erased contract is left out. The read is
// recorded in the archive audit log (DCS-FR-CSA-18).
func (s *contractStorageArchivesrvc) Evidence(ctx context.Context, p *contractstoragearchive.EvidencePayload) (res []*contractstoragearchive.ArchiveEvidenceItem, err error) {
	defer func() { s.auditFailure(ctx, "evidence", p.Did, err) }()

	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	if s.Artifacts == nil {
		return nil, contractstoragearchive.MakeInternalError(errors.New("artifact store is not configured"))
	}
	localPeer, err := s.DIDDocument.GetID()
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := authorizeContractRead(ctx, s.DB, localPeer, p.Did); err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	var rows []archiveEvidenceRow
	if err := tx.SelectContext(ctx, &rows, `
		SELECT id, did, name, media_type, description, content_hash, content_cid, size_bytes, stored_by, stored_at
		FROM contract_archive_evidence WHERE did = $1 ORDER BY stored_at, id`, p.Did); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	res = make([]*contractstoragearchive.ArchiveEvidenceItem, 0, len(rows))
	for _, row := range rows {
		content, err := s.Artifacts.Get(ctx, artifactstore.ContractScope(row.DID), row.ContentCID)
		if err != nil && !artifactstore.IsShredded(err) {
			return nil, contractstoragearchive.MakeInternalError(fmt.Errorf("read evidence %s: %w", row.ID, err))
		}
		res = append(res, &contractstoragearchive.ArchiveEvidenceItem{
			ID: row.ID, Did: row.DID, Name: row.Name, MediaType: row.MediaType, Description: row.Description,
			ContentHash: row.ContentHash, SizeBytes: row.SizeBytes, StoredBy: row.StoredBy,
			StoredAt: row.StoredAt.UTC().Format(time.RFC3339), Content: content,
		})
	}
	evt := contractevents.RetrieveArchivedEvent{
		RetrievedBy:  middleware.GetParticipantID(ctx),
		ContractDIDs: []string{p.Did},
		OccurredAt:   time.Now().UTC(),
	}
	if err := baseevent.Create(ctx, tx, evt, componenttype.ContractStorageArchive); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	return res, nil
}

// Compliance verifies every live archive entry against the archive
// compliance rules and flags the entries that fail one (DCS-FR-CSA-19).
func (s *contractStorageArchivesrvc) Compliance(ctx context.Context, _ *contractstoragearchive.CompliancePayload) ([]*contractstoragearchive.ArchiveComplianceItem, error) {
	ctx, err := contractReadContext(ctx, s.DIDDocument)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	entries, err := s.CRepo.ReadArchiveEntries(ctx, tx)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	contracts, err := s.CRepo.ReadArchivedContracts(ctx, tx)
	if err != nil {
		return nil, contractstoragearchive.MakeInternalError(err)
	}
	metadata := make(map[string]db.ContractMetadata, len(contracts))
	for _, contract := range contracts {
		metadata[contract.DID] = contract
	}

	now := time.Now().UTC()
	res := []*contractstoragearchive.ArchiveComplianceItem{}
	for _, entry := range entries {
		if entry.DeletedAt != nil {
			continue
		}
		var meta *db.ContractMetadata
		if m, ok := metadata[entry.DID]; ok {
			meta = &m
		}
		item := &contractstoragearchive.ArchiveComplianceItem{
			Did:             entry.DID,
			ContractVersion: entry.ContractVersion,
			Checks:          archiveComplianceChecks(entry, meta, now),
		}
		if meta != nil {
			item.Name = meta.Name
		}
		for _, check := range item.Checks {
			if !check.Passed {
				item.Flagged = true
			}
		}
		res = append(res, item)
	}
	return res, nil
}

// archiveComplianceChecks evaluates one archive entry against the rules of
// DCS-FR-CSA-19. meta is the contract's metadata; nil when it is unavailable.
func archiveComplianceChecks(entry db.ContractArchiveEntry, meta *db.ContractMetadata, now time.Time) []*contractstoragearchive.ArchiveComplianceCheck {
	check := func(rule string, problems []string) *contractstoragearchive.ArchiveComplianceCheck {
		c := &contractstoragearchive.ArchiveComplianceCheck{Rule: rule, Passed: len(problems) == 0}
		if !c.Passed {
			reason := strings.Join(problems, "; ")
			c.Reason = &reason
		}
		return c
	}

	var retention []string
	if entry.RetentionUntil != nil && now.After(*entry.RetentionUntil) {
		retention = append(retention, "retention period ended on "+entry.RetentionUntil.UTC().Format("2006-01-02")+"; review the entry for deletion")
	}

	var signature []string
	if archiveJSONStatus(entry.SignatureMeta) != "SIGNED" {
		signature = append(signature, "no completed signature recorded")
	} else if archiveJSONField(entry.SignatureMeta, "credential_type") == "" {
		signature = append(signature, "signature credential type not recorded")
	}
	if !archiveEvidencePresent(entry.CredentialHashes) {
		signature = append(signature, "no signer credential hashes recorded")
	}

	metadata := []string{"contract metadata unavailable"}
	if meta != nil {
		var missing []string
		if meta.Name == nil || strings.TrimSpace(*meta.Name) == "" {
			missing = append(missing, "name")
		}
		if meta.Responsible == nil || meta.Responsible.Creator == "" || meta.Responsible.Counterparty == "" {
			missing = append(missing, "parties")
		}
		if meta.StartDate == nil || meta.ExpDate == nil {
			missing = append(missing, "validity period")
		}
		metadata = nil
		if len(missing) > 0 {
			metadata = []string{"missing " + strings.Join(missing, ", ")}
		}
	}

	var evidence []string
	if !entry.ContractSnapshot.IsNotNullValue() || entry.ContentHash == "" {
		evidence = append(evidence, "no contract snapshot")
	}
	if !archiveEvidencePresent(entry.TSAReceipt) {
		evidence = append(evidence, "no TSA receipt")
	}

	return []*contractstoragearchive.ArchiveComplianceCheck{
		check("retention", retention),
		check("signature_policy", signature),
		check("metadata_completeness", metadata),
		check("evidence_completeness", evidence),
	}
}
