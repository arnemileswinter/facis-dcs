package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type AcceptNegotiationCmd struct {
	ID         string
	DID        string
	AcceptedBy string
}

type NegotiationAcceptor struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
	NRepo  db.NegotiationRepo
}

func (h *NegotiationAcceptor) Handle(cmd AcceptNegotiationCmd) error {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	processData, err := h.CRepo.ReadProcessData(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not process core data: %w", err)
	}

	if processData.State != contractstate.Negotiation.String() {
		return errors.New("current contract state is invalid")
	}

	isValidCounterpart, err := h.NRepo.IsValidCounterpart(tx, cmd.DID, processData.ContractVersion, cmd.AcceptedBy)
	if err != nil {
		return fmt.Errorf("could not validate counterpart: %w", err)
	}

	if cmd.AcceptedBy != processData.CreatedBy && isValidCounterpart == false {
		return errors.New("invalid user")
	}

	err = h.NRepo.Accept(tx, cmd.ID, cmd.AcceptedBy)
	if err != nil {
		return fmt.Errorf("could not accept negotiation: %w", err)
	}

	evt := contractevents.AcceptNegotiationEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		AcceptedBy:      cmd.AcceptedBy,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
