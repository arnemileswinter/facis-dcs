package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	archive "digital-contracting-service/gen/contract_storage_archive"
	pac "digital-contracting-service/gen/process_audit_and_compliance"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/artifactstore"
	"digital-contracting-service/internal/base/datatype"
	basepg "digital-contracting-service/internal/base/db/pq"
	cwepg "digital-contracting-service/internal/contractworkflowengine/db/pg"
	erasepg "digital-contracting-service/internal/dcstodcs/db/pg"
	"digital-contracting-service/internal/middleware"
	"digital-contracting-service/internal/processauditandcompliance/auditexecutor"
)

type qaCapturingExecutor struct{ requests []auditexecutor.Request }

func (e *qaCapturingExecutor) Run(_ context.Context, request auditexecutor.Request) (auditexecutor.Response, []byte, error) {
	e.requests = append(e.requests, request)
	response := auditexecutor.Response{ContractVersion: request.ContractVersion, AuditID: request.AuditID, CorrelationID: request.CorrelationID, Scope: request.Scope, Resource: request.Resource, ExecutedAt: time.Now().UTC().Format(time.RFC3339)}
	raw, err := json.Marshal(response)
	return response, raw, err
}

func TestQAArchiveAccessBypassesPostgres(t *testing.T) {
	t.Setenv("ORCE_ARCHIVE_AUDIT_LOG_URL", "")
	database, _ := qaPostgresSchema(t)
	ctx := context.Background()
	document := jadesTestDIDDocument(t, "local")
	peer, err := document.GetID()
	require.NoError(t, err)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	objects := &qaReportObjects{blobs: map[string][]byte{}}
	keys := &artifactstore.PostgresCEKRepo{DB: database}
	store := artifactstore.New(objects, keys, qaReportAgreement{key}, peer, peer+"#agreement", &key.PublicKey)
	repo := &cwepg.PostgresContractRepo{}
	reader := base.AuditTrailReader{Artifacts: store, ARepo: &basepg.PostgresAuditTrailRepository{}}
	archiveService := &contractStorageArchivesrvc{DB: database, DIDDocument: *document, CRepo: repo, ATrailReader: reader, Keys: keys, ERepo: &erasepg.PostgresEraseRepository{DB: database}}
	for _, fixture := range []struct{ did, owner string }{{"own", "Org A"}, {"foreign", "Org B"}} {
		_, err = database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,name,exp_date) VALUES ($1,$2,$3,'EXPIRED','template',1,$1,NOW()+INTERVAL '3 days')`, fixture.did, peer, fixture.owner)
		require.NoError(t, err)
		_, err = database.Exec(`INSERT INTO contract_archive_entries (did,contract_version,stored_by,contract_snapshot,content_hash,snapshot_cid,evidence) VALUES ($1,1,$2,'{}',$3,'test-cid','{"source":"test","snapshot_hash_algorithm":"SHA-256"}')`, fixture.did, fixture.owner, "sha256:"+fmt.Sprintf("%064d", 0))
		require.NoError(t, err)
	}
	manager := middleware.InjectAuthContext(ctx, []string{"Archive Manager"}, "holder", "Org A")
	auditor := middleware.InjectAuthContext(ctx, []string{"Auditor"}, "holder", "Audit Org")
	statistics, err := archiveService.Statistics(manager, &archive.StatisticsPayload{})
	require.NoError(t, err)
	require.Equal(t, 1, statistics.ArchivedTotal)
	require.Len(t, statistics.ExpiringContracts, 1)
	require.Equal(t, "own", statistics.ExpiringContracts[0].Did)
	statistics, err = archiveService.Statistics(auditor, &archive.StatisticsPayload{})
	require.NoError(t, err)
	require.Equal(t, 2, statistics.ArchivedTotal)
	own, foreign := "own", "foreign"
	_, err = archiveService.ErasureStatus(manager, &archive.ErasureStatusPayload{Did: foreign})
	require.ErrorContains(t, err, "not authorized")
	_, err = archiveService.ErasureStatus(manager, &archive.ErasureStatusPayload{Did: own})
	require.NoError(t, err)
	_, err = archiveService.Audit(manager, &archive.AuditPayload{Did: &foreign})
	require.ErrorContains(t, err, "not authorized")
	trail, err := archiveService.Audit(manager, &archive.AuditPayload{})
	require.NoError(t, err)
	require.Len(t, trail, 1)
	require.Equal(t, own, trail[0].Did)
	executor := &qaCapturingExecutor{}
	auditService := &processAuditAndCompliancesrvc{DB: database, CRepo: repo, ATrailReader: reader, AuditExecutor: executor}
	_, err = auditService.Audit(manager, &pac.PACAuditRequest{Scope: "archive", Did: &foreign, Justification: "QA"})
	require.ErrorContains(t, err, "not authorized")
	require.Empty(t, executor.requests)
	_, err = auditService.Audit(manager, &pac.PACAuditRequest{Scope: "archive", Justification: "QA"})
	require.NoError(t, err)
	require.Len(t, executor.requests, 1)
	evidence := executor.requests[0].Evidence["archive"].([]*auditEvidenceResource)
	require.Len(t, evidence, 1)
	require.Equal(t, own, evidence[0].Did)
	scope := "archive"
	ownReport, err := auditService.AuditReport(manager, &pac.AuditReportPayload{Scope: &scope})
	require.NoError(t, err)
	require.NotEmpty(t, ownReport)
	_, err = auditService.Audit(auditor, &pac.PACAuditRequest{Scope: "archive", Justification: "QA global"})
	require.NoError(t, err)
	report, err := auditService.AuditReport(manager, &pac.AuditReportPayload{Scope: &scope})
	require.ErrorContains(t, err, "not authorized")
	require.Empty(t, report)
	report, err = auditService.AuditReport(auditor, &pac.AuditReportPayload{Scope: &scope})
	require.NoError(t, err)
	require.NotEmpty(t, report)
	// Authorization examines identities even when no payload needs a contract encryption key.
	for _, resource := range []persistedAuditEvidenceResource{
		{DID: foreign},
		{DID: foreign, AuditTrail: []persistedAuditEvidenceEntry{{EventType: "KEY_SHREDDED", EventData: map[string]any{"erased": true}}}},
		{DID: own, AuditTrail: []persistedAuditEvidenceEntry{{DID: foreign}}},
		{DID: ""},
		{DID: "unknown"},
	} {
		request, err := json.Marshal(map[string]any{"scope": "archive", "evidence": map[string]any{"archive": []persistedAuditEvidenceResource{resource}}})
		require.NoError(t, err)
		require.ErrorContains(t, auditService.authorizeAuditReport(manager, persistedAuditRun{RequestBytes: request, ResponseBytes: []byte(`{}`)}), "not authorized")
		require.NoError(t, auditService.authorizeAuditReport(auditor, persistedAuditRun{RequestBytes: request, ResponseBytes: []byte(`{}`)}))
	}
}

func TestArchiveRecentActionsFilterBeforeLimit(t *testing.T) {
	own, foreign := "own", "foreign"
	entries := []datatype.AuditLogEntry{{DID: &own, EventType: "OWN", CreatedAt: time.Now().Add(-time.Hour)}}
	for range archiveRecentActionLimit + 1 {
		entries = append(entries, datatype.AuditLogEntry{DID: &foreign, CreatedAt: time.Now()})
	}
	entries = append(entries, datatype.AuditLogEntry{CreatedAt: time.Now()})
	actions := recentArchiveActions(filterContractAuditChains([][]datatype.AuditLogEntry{entries}, map[string]bool{own: true}), archiveRecentActionLimit)
	require.Len(t, actions, 1)
	require.Equal(t, own, actions[0].Did)
}
