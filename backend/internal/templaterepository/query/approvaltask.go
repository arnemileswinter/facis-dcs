package query

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	aopprovaltaskstate "digital-contracting-service/internal/templaterepository/datatype/approvaltaskstate"
	"digital-contracting-service/internal/templaterepository/db"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type GetAllApprovalTasksForDIDQry struct {
	DID         string
	RetrievedBy string
}

type GetAllApprovalTasksForDIDResult struct {
	ID          int
	DID         string
	State       aopprovaltaskstate.ApprovalTaskState
	Approver    string
	CreatedBy   string
	CreatedAt   time.Time
	CancelledAt *time.Time
}

type GetAllApprovalTasksForDIDHandler struct {
	Ctx    context.Context
	DB     *sqlx.DB
	ATRepo db.ApprovalTaskRepo
}

func (h *GetAllApprovalTasksForDIDHandler) Handle(query GetAllApprovalTasksForDIDQry) ([]GetAllApprovalTasksForDIDResult, error) {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	reviewTasks, err := h.ATRepo.ReadAll(tx, query.DID)
	if err != nil {
		return nil, fmt.Errorf("could not read all review tasks: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	result := make([]GetAllApprovalTasksForDIDResult, len(reviewTasks))
	for i, data := range reviewTasks {

		state, err := aopprovaltaskstate.NewApprovalTaskState(data.State)
		if err != nil {
			return nil, fmt.Errorf("could not create approval task state: %w", err)
		}

		result[i] = GetAllApprovalTasksForDIDResult{
			DID:       data.DID,
			State:     state,
			Approver:  data.Approver,
			CreatedBy: data.CreatedBy,
			CreatedAt: data.CreatedAt,
		}
	}

	return result, nil
}
