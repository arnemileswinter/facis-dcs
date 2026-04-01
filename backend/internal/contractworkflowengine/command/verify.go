package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/reviewtaskstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	templateevents "digital-contracting-service/internal/contractworkflowengine/event"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type VerifyCmd struct {
	DID        string
	UpdatedAt  time.Time
	VerifiedBy string
}

type Verifier struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
}

func (h *Verifier) Handle(cmd VerifyCmd) error {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	processData, err := h.CRepo.ReadProcessData(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	if cmd.UpdatedAt.Unix() < processData.UpdatedAt.Unix() {
		return errors.New("contract was updated elsewhere, please reload")
	}

	hasTask, err := h.RTRepo.TaskExistsInState(tx, cmd.DID, cmd.VerifiedBy, reviewtaskstate.Open.String())
	if err != nil {
		return err
	}

	if hasTask {
		err := h.RTRepo.UpdateState(tx, cmd.DID, cmd.VerifiedBy, reviewtaskstate.Verified.String())
		if err != nil {
			return err
		}
	}

	evt := templateevents.VerifyEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		VerifiedBy:      cmd.VerifiedBy,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
