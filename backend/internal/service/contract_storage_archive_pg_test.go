package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	contractstoragearchive "digital-contracting-service/gen/contract_storage_archive"
	"digital-contracting-service/internal/base/artifactstore"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/contractworkflowengine/db/pg"
	"digital-contracting-service/internal/middleware"
)

const archiveManager = "did:web:dcs.example:archive-manager"

func archiveTestService(db *sqlx.DB) *contractStorageArchivesrvc {
	return &contractStorageArchivesrvc{DB: db, CRepo: &pg.PostgresContractRepo{}}
}

func archiveManagerContext() context.Context {
	return middleware.InjectAuthContext(context.Background(), []string{"ARCHIVE_MANAGER"}, archiveManager, archiveManager)
}

// seedArchivedContract writes a contract in the given state with an archive
// entry for its current version.
func seedArchivedContract(t *testing.T, db *sqlx.DB, did, state string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO contracts (did, origin, created_by, state, template_did, template_version, name, contract_data)
		VALUES ($1, 'did:web:dcs.example', 'did:web:creator', $2, 'did:web:tpl', 1, $3, '{"clause":"terms"}')`, did, state, "Archived "+did)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO contract_archive_entries (did, contract_version, stored_by, contract_snapshot, content_hash, snapshot_cid, evidence)
		VALUES ($1, 1, 'did:web:creator', '{"did":"x"}', 'sha256:`+zeroHash+`', 'bafy-snapshot',
		        '{"source":"SIGNING_WORKFLOW_COMPLETION","snapshot_hash_algorithm":"SHA-256"}')`, did)
	require.NoError(t, err)
}

const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// archiveEvents returns the event payloads of the given type, oldest first.
func archiveEvents(t *testing.T, db *sqlx.DB, eventType string) []map[string]any {
	t.Helper()
	var rows []string
	require.NoError(t, db.Select(&rows, `SELECT event_data::text FROM outbox_events WHERE event_type = $1 ORDER BY id`, eventType))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(row), &data))
		out = append(out, data)
	}
	return out
}

func TestArchiveRetrievalIsAuditedWithTheContractIDs(t *testing.T) {
	db := pgtest.Open(t)
	seedArchivedContract(t, db, "did:web:dcs.example:contract:a", "SIGNED")
	svc := archiveTestService(db)

	_, err := svc.Retrieve(archiveManagerContext(), &contractstoragearchive.ArchiveRetrieveRequest{})
	require.NoError(t, err)
	retrieved := archiveEvents(t, db, "RETRIEVE_ARCHIVED_CONTRACTS")
	require.Len(t, retrieved, 1)
	require.Equal(t, archiveManager, retrieved[0]["retrieved_by"])
	require.Equal(t, []any{"did:web:dcs.example:contract:a"}, retrieved[0]["contract_dids"])

	did := "did:web:dcs.example:contract:a"
	_, err = svc.Search(archiveManagerContext(), &contractstoragearchive.ArchiveSearchRequest{Did: &did})
	require.NoError(t, err)
	searched := archiveEvents(t, db, "SEARCH_ARCHIVED_CONTRACTS")
	require.Len(t, searched, 1)
	require.Equal(t, []any{did}, searched[0]["contract_dids"])
	require.Equal(t, map[string]any{"did": did}, searched[0]["criteria"])
}

func TestFailedArchiveOperationsAreAuditedWithTheirOutcome(t *testing.T) {
	db := pgtest.Open(t)
	svc := archiveTestService(db)

	_, err := svc.Delete(archiveManagerContext(), &contractstoragearchive.DeletePayload{Did: "did:web:dcs.example:contract:none", Justification: "retention over"})
	require.Error(t, err)
	bad := "not-a-date"
	_, err = svc.Search(archiveManagerContext(), &contractstoragearchive.ArchiveSearchRequest{ValidFrom: &bad})
	require.Error(t, err)

	failed := archiveEvents(t, db, "ARCHIVE_OPERATION_FAILED")
	require.Len(t, failed, 2)
	require.Equal(t, "delete", failed[0]["operation"])
	require.Equal(t, "did:web:dcs.example:contract:none", failed[0]["did"])
	require.Equal(t, archiveManager, failed[0]["actor"])
	require.Equal(t, "FAILURE", failed[0]["outcome"])
	require.Equal(t, "search", failed[1]["operation"])
}

// Access attempts are part of the immutable audit log: the database refuses
// to change or remove them.
func TestAccessAttemptsAreAppendOnly(t *testing.T) {
	db := pgtest.Open(t)
	_, err := db.Exec(`INSERT INTO access_attempts (attempt_by, ip_address, attempted_at, success, service, method) VALUES ('u', '10.0.0.1', now(), false, 'ContractStorageArchive', 'retrieve')`)
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE access_attempts SET success = true`)
	require.ErrorContains(t, err, "append-only")
	_, err = db.Exec(`DELETE FROM access_attempts`)
	require.ErrorContains(t, err, "append-only")
}

// DCS-FR-CSA-09: the identifiers of an archived contract are persistent and
// immutable.
func TestArchivedContractIdentifiersAreImmutable(t *testing.T) {
	db := pgtest.Open(t)
	did := "did:web:dcs.example:contract:ids"
	seedArchivedContract(t, db, did, "SIGNED")

	_, err := db.Exec(`UPDATE contract_archive_entries SET id = gen_random_uuid() WHERE did = $1`, did)
	require.ErrorContains(t, err, "immutable", "the archive entry UUID changed")
	_, err = db.Exec(`UPDATE contract_archive_entries SET did = 'did:web:dcs.example:contract:other' WHERE did = $1`, did)
	require.ErrorContains(t, err, "immutable", "the archive entry DID changed")
	_, err = db.Exec(`UPDATE contracts SET did = 'did:web:dcs.example:contract:other' WHERE did = $1`, did)
	require.Error(t, err, "the archived contract's DID changed")
}

// DCS-FR-CSA-16: a terminated contract stays in the archive, marked
// TERMINATED, and its entry stays read-only.
func TestTerminatedContractStaysInTheArchiveReadOnly(t *testing.T) {
	db := pgtest.Open(t)
	did := "did:web:dcs.example:contract:ended"
	seedArchivedContract(t, db, did, "SIGNED")
	_, err := db.Exec(`UPDATE contracts SET state = 'TERMINATED' WHERE did = $1`, did)
	require.NoError(t, err)

	res, err := archiveTestService(db).Retrieve(archiveManagerContext(), &contractstoragearchive.ArchiveRetrieveRequest{})
	require.NoError(t, err)
	require.Len(t, res.Contracts, 1)
	require.Equal(t, did, res.Contracts[0].Did)
	require.Equal(t, "TERMINATED", res.Contracts[0].State)

	_, err = db.Exec(`UPDATE contract_archive_entries SET contract_snapshot = '{"forged":true}'::jsonb WHERE did = $1`, did)
	require.ErrorContains(t, err, "immutable")
}

// memoryArtifacts stands in for the encrypting IPFS store.
type memoryArtifacts struct {
	blobs    map[string][]byte
	shredded map[string]bool
}

func newMemoryArtifacts() *memoryArtifacts {
	return &memoryArtifacts{blobs: map[string][]byte{}, shredded: map[string]bool{}}
}

func (m *memoryArtifacts) Put(_ context.Context, _ artifactstore.Scope, plaintext []byte) (string, error) {
	cid := fmt.Sprintf("bafy-%d", len(m.blobs)+1)
	m.blobs[cid] = append([]byte(nil), plaintext...)
	return cid, nil
}

func (m *memoryArtifacts) Get(_ context.Context, scope artifactstore.Scope, cid string) ([]byte, error) {
	if m.shredded[scope.ID] {
		return nil, &artifactstore.ShreddedError{Scope: scope}
	}
	return m.blobs[cid], nil
}

func TestArchiveManagerStoresAFinalContractOnce(t *testing.T) {
	db := pgtest.Open(t)
	did := "did:web:dcs.example:contract:terminated"
	_, err := db.Exec(`INSERT INTO contracts (did, origin, created_by, state, template_did, template_version, contract_data)
		VALUES ($1, 'did:web:dcs.example', 'did:web:creator', 'TERMINATED', 'did:web:tpl', 1, '{"clause":"terms"}')`, did)
	require.NoError(t, err)
	svc := archiveTestService(db)
	artifacts := newMemoryArtifacts()
	svc.Artifacts = artifacts

	res, err := svc.Store(archiveManagerContext(), &contractstoragearchive.StorePayload{Kind: "contract", Did: did})
	require.NoError(t, err)
	require.Equal(t, 1, *res.ContractVersion)

	var entry struct {
		StoredBy    string `db:"stored_by"`
		SnapshotCID string `db:"snapshot_cid"`
		Source      string `db:"source"`
	}
	require.NoError(t, db.Get(&entry, `SELECT stored_by, snapshot_cid, evidence->>'source' AS source FROM contract_archive_entries WHERE did = $1`, did))
	require.Equal(t, archiveManager, entry.StoredBy)
	require.Equal(t, "ARCHIVE_MANAGER_STORE", entry.Source)
	require.Contains(t, string(artifacts.blobs[entry.SnapshotCID]), `"clause":"terms"`)
	require.Len(t, archiveEvents(t, db, "STORE_ARCHIVED_CONTRACT"), 1)

	_, err = svc.Store(archiveManagerContext(), &contractstoragearchive.StorePayload{Kind: "contract", Did: did})
	require.ErrorContains(t, err, "already archived")
}

func TestArchiveManagerCannotArchiveAContractInWork(t *testing.T) {
	db := pgtest.Open(t)
	did := "did:web:dcs.example:contract:draft"
	_, err := db.Exec(`INSERT INTO contracts (did, origin, created_by, state, template_did, template_version) VALUES ($1, 'did:web:dcs.example', 'did:web:creator', 'DRAFT', 'did:web:tpl', 1)`, did)
	require.NoError(t, err)
	svc := archiveTestService(db)
	svc.Artifacts = newMemoryArtifacts()

	_, err = svc.Store(archiveManagerContext(), &contractstoragearchive.StorePayload{Kind: "contract", Did: did})
	require.ErrorContains(t, err, "only SIGNED, TERMINATED or EXPIRED")
	require.Len(t, archiveEvents(t, db, "ARCHIVE_OPERATION_FAILED"), 1)
}

func TestArchiveManagerStoresEvidenceThatIsReadableUntilErasure(t *testing.T) {
	db := pgtest.Open(t)
	did := "did:web:dcs.example:contract:evidence"
	seedArchivedContract(t, db, did, "SIGNED")
	svc := archiveTestService(db)
	artifacts := newMemoryArtifacts()
	svc.Artifacts = artifacts

	name, mediaType := "delivery-note.pdf", "application/pdf"
	res, err := svc.Store(archiveManagerContext(), &contractstoragearchive.StorePayload{
		Kind: "evidence", Did: did, Name: &name, MediaType: &mediaType, Content: []byte("%PDF-1.7 note"),
	})
	require.NoError(t, err)
	require.NotNil(t, res.EvidenceID)
	require.Len(t, archiveEvents(t, db, "STORE_ARCHIVE_EVIDENCE"), 1)

	items, err := svc.Evidence(archiveManagerContext(), &contractstoragearchive.EvidencePayload{Did: did})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []byte("%PDF-1.7 note"), items[0].Content)
	require.Equal(t, res.ContentHash, items[0].ContentHash)

	artifacts.shredded[did] = true
	items, err = svc.Evidence(archiveManagerContext(), &contractstoragearchive.EvidencePayload{Did: did})
	require.NoError(t, err)
	require.Nil(t, items[0].Content, "erased evidence content must not be returned")

	_, err = db.Exec(`DELETE FROM contract_archive_evidence`)
	require.ErrorContains(t, err, "append-only")
}

func TestEvidenceNeedsALiveArchiveEntry(t *testing.T) {
	db := pgtest.Open(t)
	svc := archiveTestService(db)
	svc.Artifacts = newMemoryArtifacts()
	name, mediaType := "note.txt", "text/plain"

	_, err := svc.Store(archiveManagerContext(), &contractstoragearchive.StorePayload{
		Kind: "evidence", Did: "did:web:dcs.example:contract:none", Name: &name, MediaType: &mediaType, Content: []byte("x"),
	})
	require.ErrorContains(t, err, "no live archive entry")
}
