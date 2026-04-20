package contract

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type ReviewCmd struct {
	DID        string
	ReviewedBy string
}

type Reviewer struct {
	DB    *sqlx.DB
	CRepo db.ContractRepo
}

func (h *Reviewer) Handle(ctx context.Context, cmd ReviewCmd) error {

	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	evt := contractevents.ReviewEvent{
		DID:        cmd.DID,
		ReviewedBy: cmd.ReviewedBy,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
