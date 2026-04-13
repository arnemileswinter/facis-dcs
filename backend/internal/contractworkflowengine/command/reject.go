package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/approvaltaskstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type RejectCmd struct {
	DID        string
	UpdatedAt  time.Time
	RejectedBy string
	Reason     string
}

type Rejecter struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
	ATRepo db.ApprovalTaskRepo
}

func (h *Rejecter) Handle(cmd RejectCmd) error {

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

	if processData.State != contractstate.Reviewed.String() || processData.State == contractstate.Terminated.String() {
		return errors.New("invalid contract state")
	}

	exist, err := h.ATRepo.IsValidApprover(tx, cmd.DID, cmd.RejectedBy)
	if err != nil {
		return err
	}

	if !exist {
		return errors.New("invalid user")
	}

	err = h.ATRepo.UpdateState(tx, cmd.DID, cmd.RejectedBy, approvaltaskstate.Rejected.String())
	if err != nil {
		return fmt.Errorf("could not update approval task state: %w", err)
	}

	err = h.CRepo.UpdateState(tx, cmd.DID, contractstate.Rejected.String())
	if err != nil {
		return fmt.Errorf("could not update current state: %w", err)
	}

	evt := contractevents.RejectEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		RejectedBy:      cmd.RejectedBy,
		Reason:          cmd.Reason,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	err = h.RTRepo.Delete(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not delete review tasks: %w", err)
	}

	err = h.ATRepo.Delete(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not delete approval tasks: %w", err)
	}

	return tx.Commit()
}
