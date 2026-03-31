package test

import (
	"context"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/contractworkflowengine/datatype/approvaltaskstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/query/contract"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreate_RejectContractInReviewedState(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	creator := "Test User"

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContract(t, db, repo, did, contractstate.Reviewed, creator)

	approver := "Test User 1"

	createApprovalTasks(t, ctx, db, repo, *did, approvaltaskstate.Open, creator, approver)

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		RejectedBy: approver,
		Reason:     "Test Reason",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to submit contract : %v", err)
	}

	retrievedBy := "Test User"

	qry := contract.GetByIDQry{
		DID:         *did,
		RetrievedBy: retrievedBy,
	}
	queryHandler := contract.GetByIDHandler{
		Ctx:   ctx,
		DB:    db,
		CRepo: repo.CRepo,
		NRepo: repo.NRepo,
	}
	contract, err := queryHandler.Handle(qry)
	if err != nil {
		t.Fatalf("Failed to query contract : %v", err)
	}

	assert.Equal(t, contractstate.Draft, contract.State)
}

func TestCreate_RejectContractInReviewedStateWithInvalidUser(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	creator := "Test User"

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContract(t, db, repo, did, contractstate.Reviewed, creator)

	createApprovalTasks(t, ctx, db, repo, *did, approvaltaskstate.Open, creator, "Test User 1")

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		RejectedBy: "Test User 2",
		Reason:     "Test Reason",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestCreate_RejectNonExistingContract(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		RejectedBy: "Test User 1",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestCreate_RejectContractInDraftState(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContract(t, db, repo, did, contractstate.Draft, "Test User")

	rejectedBy := "Test User"

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		RejectedBy: rejectedBy,
		Reason:     "Test Reason",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestCreate_RejectContractInApprovedState(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContract(t, db, repo, did, contractstate.Approved, "Test User")

	rejectedBy := "Test User"

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		RejectedBy: rejectedBy,
		Reason:     "Test Reason",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestCreate_RejectContractAfterUpdate(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContract(t, db, repo, did, contractstate.Reviewed, "Test User")

	rejectedBy := "Test User"

	cmd := command.RejectCmd{
		DID:        *did,
		UpdatedAt:  time.Now().Add(-5 * time.Second),
		RejectedBy: rejectedBy,
		Reason:     "Test Reason",
	}
	handler := command.Rejecter{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		ATRepo: repo.ATRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}
