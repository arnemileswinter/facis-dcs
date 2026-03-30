package test

import (
	"context"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/query/contract"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdate_UpdateContractDataInDraftState(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Draft, creator)

	contractData := map[string]interface{}{
		"test": "update",
	}
	jsonContractData, err := datatype.NewJSON(contractData)
	if err != nil {
		t.Fatalf("Failed to create JSON  data: %v", err)
	}

	name := "Updated Contract"
	description := "Updated Description"

	cmd := command.UpdateCmd{
		DID:          *did,
		UpdatedBy:    creator,
		UpdatedAt:    time.Now(),
		Name:         &name,
		Description:  &description,
		ContractData: &jsonContractData,
	}
	handler := command.Updater{
		Ctx:   ctx,
		DB:    db,
		CRepo: repo.CRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		t.Fatalf("Failed to submit contract: %v", err)
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
	result, err := queryHandler.Handle(qry)
	if err != nil {
		t.Fatalf("Failed to query contract: %v", err)
	}

	assert.Equal(t, *did, result.DID)
	assert.Equal(t, name, *result.Name)
	assert.Equal(t, description, *result.Description)
	//assert.Equal(t, jsonContractData, result.ContractData)
}

func TestUpdate_UpdateNonExistingContract(t *testing.T) {

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

	cmd := command.UpdateCmd{
		DID:       *did,
		UpdatedAt: time.Now(),
		UpdatedBy: "Test User 1",
	}
	handler := command.Updater{
		Ctx:   ctx,
		DB:    db,
		CRepo: repo.CRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestUpdate_UpdateContractDataInDraftStateWithInvalidUser(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Draft, creator)

	contractData := map[string]interface{}{
		"test": "update",
	}
	jsonContractData, err := datatype.NewJSON(contractData)
	if err != nil {
		t.Fatalf("Failed to create JSON data: %v", err)
	}

	name := "Updated Contract"
	description := "Updated Description"

	cmd := command.UpdateCmd{
		DID:          *did,
		UpdatedBy:    "Test User 1",
		UpdatedAt:    time.Now(),
		Name:         &name,
		Description:  &description,
		ContractData: &jsonContractData,
	}
	handler := command.Updater{
		Ctx:   ctx,
		DB:    db,
		CRepo: repo.CRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}

func TestUpdate_UpdateContractDataInInvalidState(t *testing.T) {

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

	createContract(t, db, repo, did, contractstate.Submitted, creator)

	contractData := map[string]interface{}{
		"test": "update",
	}
	jsonContractData, err := datatype.NewJSON(contractData)
	if err != nil {
		t.Fatalf("Failed to create JSON data: %v", err)
	}

	name := "Updated Contract"
	description := "Updated Description"

	cmd := command.UpdateCmd{
		DID:          *did,
		UpdatedBy:    creator,
		UpdatedAt:    time.Now(),
		Name:         &name,
		Description:  &description,
		ContractData: &jsonContractData,
	}
	handler := command.Updater{
		Ctx:   ctx,
		DB:    db,
		CRepo: repo.CRepo,
	}
	err = handler.Handle(cmd)

	assert.NotNil(t, err)
}
