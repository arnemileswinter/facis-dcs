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
	NTRepo db.NegotiationRepo
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

	if processData.State != contractstate.Negotiation.String() {
		return errors.New("current contract state is invalid")
	}

	isValidReviewer, err := h.RTRepo.IsValidReviewer(tx, cmd.DID, cmd.NegotiatedBy)
	if err != nil {
		return fmt.Errorf("could not validate negotiator as reviewer: %w", err)
	}

	if cmd.NegotiatedBy != processData.CreatedBy && isValidReviewer == false {
		return errors.New("invalid user")
	}

	counterparts, err := h.RTRepo.ReadReviewersForDID(tx, cmd.DID)
	for idx, _ := range counterparts {
		if counterparts[idx] == cmd.NegotiatedBy {
			counterparts[idx] = processData.CreatedBy
			break
		}
	}

	data := db.NegotiationCreateData{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		ChangeRequest:   cmd.ChangeRequest,
		CreatedBy:       cmd.NegotiatedBy,
	}
	_, err = h.NTRepo.Create(tx, data, counterparts)
	if err != nil {
		return fmt.Errorf("could not create negotiation: %w", err)
	}

	evt := contractevents.NegotiationEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		ChangeRequest:   cmd.ChangeRequest,
		NegotiatedBy:    cmd.NegotiatedBy,
		Counterparts:    counterparts,
		OccurredAt:      time.Now(),
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
