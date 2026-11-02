package query

import (
	"context"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/event/outboxtest"
	"digital-contracting-service/internal/signingmanagement/db"
)

type validateRepoFake struct {
	db.ContractRepo
	findingsErr error
}

func (r validateRepoFake) ReadProcessDataByDID(context.Context, *sqlx.Tx, string) (*db.ContractProcessData, error) {
	return &db.ContractProcessData{ContractVersion: 3}, nil
}
func (r validateRepoFake) CollectValidationFindings(context.Context, *sqlx.Tx, string) ([]string, error) {
	return []string{"Signature of did:web:a is valid"}, r.findingsErr
}
func (r validateRepoFake) LoadSignatures(context.Context, *sqlx.Tx, string) ([]db.SignatureRecord, error) {
	return []db.SignatureRecord{{SignerDID: "did:web:a", Status: "SIGNED"}, {SignerDID: "did:web:b", Status: "REVOKED"}}, nil
}

func validateQry() ValidateQry {
	return ValidateQry{DID: "did:web:example.test:contracts:1", ValidatedBy: "participant-1"}
}

func committedValidateEvent(t *testing.T, rec *outboxtest.Recorder) outboxtest.Event {
	t.Helper()
	events := rec.Committed()
	if len(events) != 1 {
		t.Fatalf("committed outbox events = %+v, want exactly one", events)
	}
	return events[0]
}

func TestValidatorAuditsSignersAndOutcome(t *testing.T) {
	t.Setenv("DSS_URL", "")
	sqlDB, rec := outboxtest.Open(t)
	validator := Validator{DB: sqlDB, CRepo: validateRepoFake{}}

	if _, err := validator.Handle(context.Background(), validateQry()); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	evt := committedValidateEvent(t, rec)
	if evt.Type != "VALIDATE_SIGNATURE" {
		t.Fatalf("event type = %q, want VALIDATE_SIGNATURE", evt.Type)
	}
	if evt.Data["outcome"] != "COMPLETED" || evt.Data["validated_by"] != "participant-1" || evt.Data["occurred_at"] == nil {
		t.Fatalf("event data = %#v, want outcome COMPLETED, actor and timestamp", evt.Data)
	}
	signers, _ := evt.Data["signer_dids"].([]any)
	if len(signers) != 2 || signers[0] != "did:web:a" || signers[1] != "did:web:b" {
		t.Errorf("signer_dids = %#v, want both signers", evt.Data["signer_dids"])
	}
	findings, _ := evt.Data["findings"].([]any)
	if len(findings) != 1 || findings[0] != "Signature of did:web:a is valid" {
		t.Errorf("findings = %#v, want the validation result", evt.Data["findings"])
	}
}

// A failed validation rolls back its VALIDATE_SIGNATURE entry, so the failure
// must commit an entry of its own.
func TestValidatorAuditsFailedAttempts(t *testing.T) {
	t.Setenv("DSS_URL", "")
	sqlDB, rec := outboxtest.Open(t)
	cause := errors.New("database unavailable")
	validator := Validator{DB: sqlDB, CRepo: validateRepoFake{findingsErr: cause}}

	_, err := validator.Handle(context.Background(), validateQry())
	if !errors.Is(err, cause) {
		t.Fatalf("Handle() error = %v, want %v", err, cause)
	}

	evt := committedValidateEvent(t, rec)
	if evt.Type != "VALIDATE_SIGNATURE_FAILED" {
		t.Fatalf("event type = %q, want VALIDATE_SIGNATURE_FAILED", evt.Type)
	}
	if evt.Data["outcome"] != "FAILED" || evt.Data["error"] != err.Error() || evt.Data["validated_by"] != "participant-1" {
		t.Fatalf("event data = %#v, want outcome FAILED, error and actor", evt.Data)
	}
}
