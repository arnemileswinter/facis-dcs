package command

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"digital-contracting-service/internal/fcasset"
	fcclient "digital-contracting-service/internal/templatecatalogueintegration/client"
)

// ErrNotPublishable is returned for a contract that is not approved, signed or
// active.
var ErrNotPublishable = errors.New("only an approved, signed or active contract can be registered in the catalogue")

var publishableStates = []string{contractstate.Approved.String(), contractstate.Signed.String(), contractstate.Active.String()}

type CataloguePublishCmd struct {
	DID         string
	PublishedBy string
	HolderDID   string
	// InstanceDID is this DCS deployment's DID, the issuer of the catalogue
	// entry.
	InstanceDID string
	UserRoles   userrole.UserRoles
}

type CataloguePublisher struct {
	DB       *sqlx.DB
	CRepo    db.ContractRepo
	FCClient *fcclient.FederatedCatalogueClient
}

// Handle registers the contract metadata in the Federated Catalogue and
// records the registration in the audit trail (DCS-IR-CWE-10). The catalogue
// entry holds no contract content and no party.
func (h *CataloguePublisher) Handle(ctx context.Context, cmd CataloguePublishCmd) (string, error) {
	if h.FCClient == nil {
		return "", fcclient.ErrFederatedCatalogueNotConfigured
	}
	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	contract, err := h.CRepo.ReadDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return "", fmt.Errorf("could not read contract: %w", err)
	}
	if !slices.Contains(publishableStates, contract.State) {
		return "", ErrNotPublishable
	}
	name := ""
	if contract.Name != nil {
		name = *contract.Name
	}
	payload, err := fcasset.BuildContractPayload(cmd.InstanceDID, base.ResourceIRI("contract", cmd.DID), contract.UpdatedAt, fcasset.ContractSubject{
		ID:         cmd.DID,
		State:      contract.State,
		Name:       name,
		Version:    contract.ContractVersion,
		TemplateID: contract.TemplateDID,
		StartDate:  contract.StartDate,
		EndDate:    contract.ExpDate,
	})
	if err != nil {
		return "", fmt.Errorf("build contract catalogue entry: %w", err)
	}
	assetHash, err := h.FCClient.PublishAsset(ctx, payload)
	if err != nil {
		return "", fmt.Errorf("register contract in the catalogue: %w", err)
	}

	evt := contractevents.CataloguePublishEvent{
		DID:             cmd.DID,
		HolderDID:       cmd.HolderDID,
		ContractVersion: contract.ContractVersion,
		AssetHash:       assetHash,
		PublishedBy:     cmd.PublishedBy,
		OccurredAt:      time.Now().UTC(),
		UserRoles:       cmd.UserRoles,
	}
	if err := event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine); err != nil {
		return "", fmt.Errorf("could not create event: %w", err)
	}
	return assetHash, tx.Commit()
}
