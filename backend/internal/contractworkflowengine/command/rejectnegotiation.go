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

type RejectNegotiationCmd struct {
	ID              int
	DID             string
	ContractVersion *int
	RejectedBy      string
	RejectReason    *string
}

type NegotiationRejector struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
	NRepo  db.NegotiationRepo
}

func (h *NegotiationRejector) Handle(cmd RejectNegotiationCmd) error {

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

	isValidNegotiator, err := h.NRepo.IsValidNegotiator(tx, cmd.DID, cmd.ContractVersion, cmd.RejectedBy)
	if err != nil {
		return fmt.Errorf("could not validate negotiator: %w", err)
	}

	if cmd.RejectedBy != processData.CreatedBy && !isValidNegotiator {
		return errors.New("invalid user")
	}

	err = h.NRepo.Reject(tx, cmd.ID, cmd.RejectedBy, cmd.RejectReason)
	if err != nil {
		return fmt.Errorf("could not reject negotiation %w", err)
	}

	evt := contractevents.RejectNegotiationEvent{
		DID:             cmd.DID,
		ContractVersion: cmd.ContractVersion,
		RejectedBy:      cmd.RejectedBy,
		RejectionReason: cmd.RejectReason,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
