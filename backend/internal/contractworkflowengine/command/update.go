package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type UpdateCmd struct {
	DID             string
	ContractVersion *int
	UpdatedAt       time.Time
	UpdatedBy       string
	Name            *string
	Description     *string
	ContractData    *datatype.JSON
}

type Updater struct {
	Ctx   context.Context
	DB    *sqlx.DB
	CRepo db.ContractRepo
}

func (h *Updater) Handle(cmd UpdateCmd) error {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	oldData, err := h.CRepo.ReadDataByID(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read contract data: %w", err)
	}

	if cmd.UpdatedAt.Unix() < oldData.UpdatedAt.Unix() {
		return errors.New("contract was updated elsewhere, please reload")
	}

	if oldData.CreatedBy != cmd.UpdatedBy {
		return errors.New("invalid user")
	}

	if oldData.State != contracttemplatestate.Draft.String() {
		return errors.New("invalid contract state")
	}

	newData := db.ContractUpdateData{
		DID:             cmd.DID,
		ContractVersion: cmd.ContractVersion,
		Name:            cmd.Name,
		Description:     cmd.Description,
		ContractData:    cmd.ContractData,
	}
	err = h.CRepo.Update(tx, newData)
	if err != nil {
		return fmt.Errorf("could not update contract data: %w", err)
	}

	evt := contractevents.UpdateEvent{
		DID:                cmd.DID,
		OldContractVersion: oldData.ContractVersion,
		NewContractVersion: cmd.ContractVersion,
		OldName:            oldData.Name,
		NewName:            cmd.Name,
		OldDescription:     oldData.Description,
		NewDescription:     cmd.Description,
		OldContractData:    oldData.ContractData,
		NewContractData:    cmd.ContractData,
		UpdatedBy:          cmd.UpdatedBy,
		OccurredAt:         time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
