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

type AuditCmd struct {
	DID       string
	AuditedBy string
}

type Auditor struct {
	Ctx   context.Context
	DB    *sqlx.DB
	CRepo db.ContractRepo
}

func (h *Auditor) Handle(cmd AuditCmd) error {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	evt := contractevents.AuditEvent{
		DID:       cmd.DID,
		AuditedBy: cmd.AuditedBy,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
