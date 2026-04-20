package contracttemplate

import (
	"context"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/templaterepository/db"
	templateevents "digital-contracting-service/internal/templaterepository/event"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type AuditCmd struct {
	DID       string
	AuditedBy string
}

type Auditor struct {
	DB     *sqlx.DB
	CTRepo db.ContractTemplateRepo
}

func (h *Auditor) Handle(ctx context.Context, cmd AuditCmd) error {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	evt := templateevents.AuditEvt{
		DID: cmd.DID,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractTemplateRepo)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}
