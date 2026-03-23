package contract

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	templateevents "digital-contracting-service/internal/templaterepository/event"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type GetAllMetadataByFilterQry struct {
	RetrievedBy string

	DID             *string
	ContractVersion *int
	Version         *int
	State           *contractstate.ContractState
	Name            *string
	Description     *string
	Filter          *string
}

type GetAllMetadataByFilterResult struct {
	DID             string
	ContractVersion *int
	State           contractstate.ContractState
	Name            *string
	Description     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	MetaData        datatype.JSON
}

type GetAllMetaDataByFilterHandler struct {
	Ctx   context.Context
	DB    *sqlx.DB
	CRepo db.ContractRepo
}

func (h *GetAllMetaDataByFilterHandler) Handle(query GetAllMetadataByFilterQry) ([]GetAllMetadataByFilterResult, error) {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create transaction: %w", err)
	}
	defer tx.Rollback()

	var state string
	if query.State != nil {
		state = query.State.String()
	}

	searchValues := db.SearchValues{
		DID:             query.DID,
		ContractVersion: query.ContractVersion,
		State:           state,
		Name:            query.Name,
		Description:     query.Description,
		Filter:          query.Filter,
	}

	contracts, err := h.CRepo.ReadAllMetaDataByFilter(tx, searchValues)
	if err != nil {
		return nil, fmt.Errorf("could not read all contract: %w", err)
	}

	evt := templateevents.RetrieveAllEvent{
		RetrievedBy: query.RetrievedBy,
		OccurredAt:  time.Now(),
	}
	err = event.Create(h.Ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return nil, fmt.Errorf("could not create event: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	result := make([]GetAllMetadataByFilterResult, len(contracts))
	for i, data := range contracts {

		contractState, err := contractstate.NewContractState(data.State)
		if err != nil {
			return nil, fmt.Errorf("could not create contract state: %w", err)
		}

		result[i] = GetAllMetadataByFilterResult{
			DID:             data.DID,
			ContractVersion: data.ContractVersion,
			State:           contractState,
			Name:            data.Name,
			Description:     data.Description,
			CreatedAt:       data.CreatedAt,
			UpdatedAt:       data.UpdatedAt,
		}
	}

	return result, nil
}
