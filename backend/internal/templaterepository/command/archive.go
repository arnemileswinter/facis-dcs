package command

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	fcclient "digital-contracting-service/internal/templatecatalogueintegration/client"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/db"
	templateevents "digital-contracting-service/internal/templaterepository/event"
)

type ArchiveCmd struct {
	DID        string
	UpdatedAt  time.Time
	ArchivedBy string
	HolderDID  string
	UserRoles  userrole.UserRoles
}

type Archiver struct {
	DB       *sqlx.DB
	CTRepo   db.ContractTemplateRepo
	RTRepo   db.ReviewTaskRepo
	ATRepo   db.ApprovalTaskRepo
	FCClient *fcclient.FederatedCatalogueClient
}

func (h *Archiver) Handle(ctx context.Context, cmd ArchiveCmd) error {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	processData, err := h.CTRepo.ReadProcessDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	// Optimistic concurrency (see command package doc / ADR-0007).
	if cmd.UpdatedAt.Unix() < processData.ContentUpdatedAt.Unix() {
		return fmt.Errorf("contract template %w, please reload", base.ErrUpdatedElsewhere)
	}

	// A registered or published template is deprecated first; a deprecated
	// template is then deleted (DCS-FR-TR-17, DCS-FR-TR-18).
	if processData.State == contracttemplatestate.Deleted.String() {
		return ErrInvalidTemplateState
	}

	// DCS-NFR-BR-09: a deprecated template is withdrawn from the catalogue first.
	if processData.State == contracttemplatestate.Published.String() && processData.CatalogueAssetHash != nil {
		if h.FCClient == nil {
			return fcclient.ErrFederatedCatalogueNotConfigured
		}
		if err := h.FCClient.RevokeAsset(ctx, *processData.CatalogueAssetHash); err != nil {
			return fmt.Errorf("could not withdraw the template from the Federated Catalogue: %w", err)
		}
	}

	if processData.State == contracttemplatestate.Registered.String() || processData.State == contracttemplatestate.Published.String() {

		err = h.CTRepo.UpdateState(ctx, tx, cmd.DID, processData.State, contracttemplatestate.Deprecated.String())
		if err != nil {
			return fmt.Errorf("could not update state: %w", err)
		}

	} else {

		err = h.CTRepo.UpdateState(ctx, tx, cmd.DID, processData.State, contracttemplatestate.Deleted.String())
		if err != nil {
			return fmt.Errorf("could not update state: %w", err)
		}
	}

	evt := templateevents.ArchiveEvent{
		DID:        cmd.DID,
		Version:    processData.Version,
		ArchivedBy: cmd.ArchivedBy,
		OccurredAt: time.Now().UTC(),
		HolderDID:  cmd.HolderDID,
		UserRoles:  cmd.UserRoles,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractTemplateRepo)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	err = h.RTRepo.Delete(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not delete review tasks: %w", err)
	}

	err = h.ATRepo.Delete(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not delete approval tasks: %w", err)
	}

	return tx.Commit()
}
