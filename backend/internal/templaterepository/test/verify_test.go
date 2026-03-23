package test

import (
	"context"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/templaterepository/command"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/datatype/reviewtaskstate"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVerify_VerifyContractTemplateAsReviewer(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTemplateTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	creator := "Test User"

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContractTemplate(t, db, repo, did, contracttemplatestate.Submitted, creator)

	reviewers := []string{"Test User 1"}
	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, reviewers)

	cmd := command.VerifyCmd{
		DID:        *did,
		VerifiedBy: reviewers[0],
		UpdatedAt:  time.Now(),
	}
	handler := command.Verifier{
		Ctx:    ctx,
		DB:     db,
		CTRepo: repo.CTRepo,
		RTRepo: repo.RTRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to verify contract template: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	exists, err := repo.RTRepo.AnyTasksInState(tx, *did, reviewtaskstate.Verified.String())
	if err != nil {
		t.Fatalf("Failed to check existence of review tasks: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatal("could not commit transaction: %w", err)
	}

	assert.True(t, exists)
}

func TestVerify_VerifyNonExistingContractTemplate(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTemplateTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	cmd := command.VerifyCmd{
		DID:        *did,
		UpdatedAt:  time.Now(),
		VerifiedBy: "Test User 1",
	}
	handler := command.Verifier{
		Ctx:    ctx,
		DB:     db,
		CTRepo: repo.CTRepo,
		RTRepo: repo.RTRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestVerify_VerifyContractTemplateAfterUpdate(t *testing.T) {

	db := setupTestDB(t)

	cleanupContractTemplateTable(t, db)

	did, err := base.GetDID()
	if err != nil {
		t.Fatalf("Failed to get new DID: %v", err)
	}

	creator := "Test User"

	tmpCtx := context.Background()
	ctx, cancel := context.WithTimeout(tmpCtx, conf.TransactionTimeout())
	defer cancel()

	repo := NewTestRepo(ctx)

	createContractTemplate(t, db, repo, did, contracttemplatestate.Submitted, creator)

	cmd := command.VerifyCmd{
		DID:        *did,
		VerifiedBy: creator,
		UpdatedAt:  time.Now().Add(-5 * time.Second),
	}
	handler := command.Verifier{
		Ctx:    ctx,
		DB:     db,
		CTRepo: repo.CTRepo,
		RTRepo: repo.RTRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}
