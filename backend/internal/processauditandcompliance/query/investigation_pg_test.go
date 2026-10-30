package qry_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	cwerepo "digital-contracting-service/internal/contractworkflowengine/db/pg"
	pacrepo "digital-contracting-service/internal/processauditandcompliance/db/pg"
	qry "digital-contracting-service/internal/processauditandcompliance/query"
	"digital-contracting-service/migrations"
)

// investigationDB opens the Postgres named by QA_DATABASE_URL with every
// migration applied. Without it the test is skipped.
func investigationDB(t *testing.T) *sqlx.DB {
	t.Helper()
	url := os.Getenv("QA_DATABASE_URL")
	if url == "" {
		t.Skip("QA_DATABASE_URL is not set")
	}
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func insertContract(t *testing.T, db *sqlx.DB, state string, startDate time.Time) string {
	t.Helper()
	did := "did:web:test:contract:" + uuid.NewString()
	if _, err := db.Exec(`INSERT INTO contracts (did, created_by, state, template_did, template_version, start_date)
        VALUES ($1, 'did:web:test', $2, 'did:web:test:template', 1, $3)`, did, state, startDate); err != nil {
		t.Fatalf("insert contract: %v", err)
	}
	return did
}

// DCS-FR-PACM-05: the monitor flags late signatures and signatures that rely
// on a revoked certificate.
func TestMonitorFlagsLateSignaturesAndInvalidCredentials(t *testing.T) {
	db := investigationDB(t)
	late := insertContract(t, db, "APPROVED", time.Now().Add(-48*time.Hour))
	notYetDue := insertContract(t, db, "APPROVED", time.Now().Add(48*time.Hour))
	revoked := insertContract(t, db, "SIGNED", time.Now().Add(-48*time.Hour))
	if _, err := db.Exec(`INSERT INTO contract_signatures (contract_did, signer_did, credential_type, status, field_name, cert_revoked_at)
        VALUES ($1, 'did:key:signer', 'AES', 'SIGNED', 'did:web:org-a', now())`, revoked); err != nil {
		t.Fatalf("insert signature: %v", err)
	}

	monitor := qry.ComplianceMonitor{DB: db, ATRepo: &cwerepo.PostgresApprovalTaskRepo{}, CRepo: &cwerepo.PostgresContractRepo{}, FRepo: &pacrepo.PostgresRiskFindingRepo{}}
	result, err := monitor.Handle(context.Background(), qry.MonitorQry{MonitoredBy: qry.SystemMonitorPrincipal})
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	found := map[string]string{}
	for _, risk := range result.Risks {
		if risk.DID == late || risk.DID == notYetDue || risk.DID == revoked {
			found[risk.DID] = risk.RiskType
		}
	}
	if found[late] != qry.RiskTypeLateSignature {
		t.Errorf("late contract: risk %q, want %s", found[late], qry.RiskTypeLateSignature)
	}
	if _, flagged := found[notYetDue]; flagged {
		t.Errorf("contract before its start date flagged as %s", found[notYetDue])
	}
	if found[revoked] != qry.RiskTypeInvalidCredential {
		t.Errorf("revoked certificate: risk %q, want %s", found[revoked], qry.RiskTypeInvalidCredential)
	}
}

// DCS-FR-PACM-05: an incident report opens a case, and the case file exports
// its findings and records the export with the justification.
func TestIncidentReportOpensACaseWithAnExportableCaseFile(t *testing.T) {
	db := investigationDB(t)
	did := insertContract(t, db, "SIGNED", time.Now())
	ctx := context.Background()

	caseID, err := (&qry.IncidentReporter{DB: db}).Handle(ctx, qry.IncidentReportQry{
		DID: did, ReportedBy: "did:web:compliance-officer",
		Findings: []qry.IncidentFinding{{RiskType: "LATE_SIGNATURE", Detail: "signed after the start date"}},
	})
	if err != nil || caseID == "" {
		t.Fatalf("incident report: case %q, err %v", caseID, err)
	}

	exporter := qry.CaseFileExporter{DB: db}
	raw, err := exporter.Handle(ctx, qry.CaseFileQry{CaseID: caseID, ExportedBy: "did:web:auditor", Justification: "regulator request"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var file qry.CaseFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("decode case file: %v", err)
	}
	if file.CaseID != caseID || file.DID != did || len(file.Findings) != 1 || file.Findings[0].RiskType != "LATE_SIGNATURE" {
		t.Fatalf("case file = %+v", file)
	}
	var justification string
	if err := db.Get(&justification, `SELECT event_data->>'justification' FROM outbox_events
        WHERE event_type = 'PAC_CASE_FILE_EXPORTED' AND event_data->>'case_id' = $1`, caseID); err != nil || justification != "regulator request" {
		t.Fatalf("export event justification %q, err %v", justification, err)
	}

	if _, err := exporter.Handle(ctx, qry.CaseFileQry{CaseID: uuid.NewString()}); !errors.Is(err, qry.ErrCaseNotFound) {
		t.Fatalf("unknown case: err %v, want ErrCaseNotFound", err)
	}
}
