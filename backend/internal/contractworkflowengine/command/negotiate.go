package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
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

type NegotiationCmd struct {
	DID           string
	NegotiatedBy  string
	ChangeRequest *datatype.JSON
	UpdatedAt     time.Time
}

type Negotiator struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
	NRepo  db.NegotiationRepo
	NTRepo db.NegotiationTaskRepo
}

func (h *Negotiator) Handle(cmd NegotiationCmd) error {

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

	if cmd.UpdatedAt.Unix() < processData.UpdatedAt.Unix() {
		return errors.New("contract was updated elsewhere, please reload")
	}

	if processData.State != contractstate.Negotiation.String() {
		return errors.New("current contract state is invalid")
	}

	isValidNegotiator, err := h.NTRepo.IsValidNegotiator(tx, cmd.DID, cmd.NegotiatedBy)
	if err != nil {
		return fmt.Errorf("could not validate negotiator: %w", err)
	}

	if isValidNegotiator == false {
		return errors.New("invalid user")
	}

	negotiators, err := h.NTRepo.ReadNegotiatorsForDID(tx, cmd.DID)
	data := db.NegotiationCreateData{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		ChangeRequest:   cmd.ChangeRequest,
		CreatedBy:       cmd.NegotiatedBy,
	}
	_, err = h.NRepo.Create(tx, data, negotiators)
	if err != nil {
		return fmt.Errorf("could not create negotiation: %w", err)
	}

	evt := contractevents.NegotiationEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		ChangeRequest:   cmd.ChangeRequest,
		NegotiatedBy:    cmd.NegotiatedBy,
		Negotiators:     negotiators,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
