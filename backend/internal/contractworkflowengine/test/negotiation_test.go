package test

import (
	"context"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/negotiationdescision"
	"digital-contracting-service/internal/contractworkflowengine/datatype/reviewtaskstate"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNegotiation_CreateNegotiation(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	result, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	assert.Equal(t, len(result), 3)
}

func TestNegotiation_CreateNegotiationAndCounterpartSet(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User 2",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	result, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	correctCounterParts := []string{
		"Test User 1",
		"Test User",
		"Test User 3",
	}
	for idx, _ := range result {
		assert.Equal(t, result[idx].Counterpart, correctCounterParts[idx])
	}

	assert.Equal(t, len(result), 3)
}

func TestNegotiation_AllCounterpartsAcceptChangeRequest(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	negotiations, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	for _, negotiation := range negotiations {
		acceptCmd := command.AcceptNegotiationCmd{
			DID:        *did,
			ID:         negotiation.ID,
			AcceptedBy: negotiation.Counterpart,
		}
		acceptHandler := command.NegotiationAcceptor{
			Ctx:    ctx,
			DB:     db,
			CRepo:  repo.CRepo,
			RTRepo: repo.RTRepo,
			NRepo:  repo.NRepo,
		}
		err := acceptHandler.Handle(acceptCmd)
		if err != nil {
			t.Fatalf("Failed to accept negotiation: %v", err)
		}
	}

	tx, err = db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	acceptAmount := 0
	rejectAmount := 0
	closeAmount := 0
	negotiations, err = repo.NRepo.ReadAllByContractDID(tx, *did)
	for _, negotiation := range negotiations {
		if *negotiation.Decision == negotiationdescision.Accepted.String() {
			acceptAmount++
		} else if *negotiation.Decision == negotiationdescision.Rejected.String() {
			rejectAmount++
		} else if *negotiation.Decision == negotiationdescision.Closed.String() {
			closeAmount++
		}
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	assert.Equal(t, acceptAmount, 3)
	assert.Equal(t, rejectAmount, 0)
	assert.Equal(t, closeAmount, 0)
}

func TestNegotiation_OneCounterpartRejectChangeRequest(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	negotiations, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	rejectionReason := "RejectionReason"
	rejectionCmd := command.RejectNegotiationCmd{
		DID:             *did,
		ID:              negotiations[0].ID,
		RejectionReason: &rejectionReason,
		RejectedBy:      negotiations[0].Counterpart,
	}
	rejectionHandler := command.NegotiationRejector{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NRepo:  repo.NRepo,
	}
	err = rejectionHandler.Handle(rejectionCmd)
	if err != nil {
		t.Fatalf("Failed to reject negotiation: %v", err)
	}

	tx, err = db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	acceptAmount := 0
	rejectAmount := 0
	closeAmount := 0
	negotiations, err = repo.NRepo.ReadAllByContractDID(tx, *did)
	for _, negotiation := range negotiations {
		if *negotiation.Decision == negotiationdescision.Accepted.String() {
			acceptAmount++
		} else if *negotiation.Decision == negotiationdescision.Rejected.String() {
			rejectAmount++
		} else if *negotiation.Decision == negotiationdescision.Closed.String() {
			closeAmount++
		}
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	assert.Equal(t, acceptAmount, 0)
	assert.Equal(t, rejectAmount, 1)
	assert.Equal(t, closeAmount, 2)
}

func TestNegotiation_OneAcceptionOneRejectionOfChangeRequest(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	negotiations, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	acceptCmd := command.AcceptNegotiationCmd{
		DID:        *did,
		ID:         negotiations[0].ID,
		AcceptedBy: negotiations[0].Counterpart,
	}
	acceptHandler := command.NegotiationAcceptor{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NRepo:  repo.NRepo,
	}
	err = acceptHandler.Handle(acceptCmd)
	if err != nil {
		t.Fatalf("Failed to accept negotiation: %v", err)
	}

	rejectionReason := "RejectionReason"
	rejectionCmd := command.RejectNegotiationCmd{
		DID:             *did,
		ID:              negotiations[1].ID,
		RejectionReason: &rejectionReason,
		RejectedBy:      negotiations[1].Counterpart,
	}
	rejectionHandler := command.NegotiationRejector{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NRepo:  repo.NRepo,
	}
	err = rejectionHandler.Handle(rejectionCmd)
	if err != nil {
		t.Fatalf("Failed to reject negotiation: %v", err)
	}

	tx, err = db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	acceptAmount := 0
	rejectAmount := 0
	closeAmount := 0
	negotiations, err = repo.NRepo.ReadAllByContractDID(tx, *did)
	for _, negotiation := range negotiations {
		if *negotiation.Decision == negotiationdescision.Accepted.String() {
			acceptAmount++
		} else if *negotiation.Decision == negotiationdescision.Rejected.String() {
			rejectAmount++
		} else if *negotiation.Decision == negotiationdescision.Closed.String() {
			closeAmount++
		}
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	assert.Equal(t, acceptAmount, 1)
	assert.Equal(t, rejectAmount, 1)
	assert.Equal(t, closeAmount, 1)
}

func TestNegotiation_TestForOpenNegationDecisions(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Negotiation, creator)

	createReviewTasks(t, ctx, db, repo, *did, reviewtaskstate.Open, creator, []string{
		"Test User 1",
		"Test User 2",
		"Test User 3",
	})

	var changeRequest map[string]interface{}
	jsonChangeRequest, err := datatype.NewJSON(changeRequest)
	cmd := command.NegotiationCmd{
		DID:           *did,
		NegotiatedBy:  "Test User",
		ChangeRequest: &jsonChangeRequest,
	}
	handler := command.Negotiator{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NTRepo: repo.NRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to create negotiation: %v", err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	negotiations, err := repo.NRepo.ReadAllByContractDID(tx, *did)
	if err != nil {
		t.Fatalf("Failed to read all negotiations for did: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	acceptCmd := command.AcceptNegotiationCmd{
		DID:        *did,
		ID:         negotiations[0].ID,
		AcceptedBy: negotiations[0].Counterpart,
	}
	acceptHandler := command.NegotiationAcceptor{
		Ctx:    ctx,
		DB:     db,
		CRepo:  repo.CRepo,
		RTRepo: repo.RTRepo,
		NRepo:  repo.NRepo,
	}
	err = acceptHandler.Handle(acceptCmd)
	if err != nil {
		t.Fatalf("Failed to accept negotiation: %v", err)
	}

	tx, err = db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	hasOpenNegationDecisions, err := repo.NRepo.HasOpenNegotiationDecisions(tx, *did, nil)
	if err != nil {
		t.Fatalf("Failed to check for open negation decisions %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	assert.Equal(t, hasOpenNegationDecisions, true)
}
