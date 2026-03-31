package pg

import (
	"context"
	"digital-contracting-service/internal/contractworkflowengine/db"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostgresApprovalTaskRepo struct {
	Ctx context.Context
}

func (r *PostgresApprovalTaskRepo) Create(tx *sqlx.Tx, data db.ApprovalTaskData) (*time.Time, error) {
	statement := `
        INSERT INTO contract_approval_task (
            did, state, approver, created_by
        ) VALUES ($1, $2, $3, $4)
        RETURNING created_at
    `
	var createdAt time.Time
	err := tx.GetContext(r.Ctx, &createdAt, statement,
		data.DID,
		data.State, data.Approver, data.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	return &createdAt, nil
}

func (r *PostgresApprovalTaskRepo) ReopenTasks(tx *sqlx.Tx, did string) error {
	statement := `
        UPDATE contract_approval_task SET state = 'OPEN'
        WHERE did = $1
    `
	_, err := tx.ExecContext(r.Ctx, statement, did)
	return err
}

func (r *PostgresApprovalTaskRepo) ReadAll(tx *sqlx.Tx, did string) ([]db.ApprovalTaskData, error) {
	query := `
        SELECT id, did, state, approver,
               created_by, created_at
        FROM contract_approval_task WHERE did = $1
    `
	var approvalTasks []db.ApprovalTaskData
	err := tx.SelectContext(r.Ctx, &approvalTasks, query, did)
	if err != nil {
		return nil, err
	}
	return approvalTasks, nil
}

func (r *PostgresApprovalTaskRepo) ReadAllByApprover(tx *sqlx.Tx, approver string) ([]db.ApprovalTaskData, error) {
	query := `
        SELECT id, did, state, approver,
               created_by, created_at
        FROM contract_approval_task WHERE approver = $1
    `
	var approvalTasks []db.ApprovalTaskData
	err := tx.SelectContext(r.Ctx, &approvalTasks, query, approver)
	if err != nil {
		return nil, err
	}
	return approvalTasks, nil
}

func (r *PostgresApprovalTaskRepo) UpdateState(tx *sqlx.Tx, did string, approver string, state string) error {
	statement := `
        UPDATE contract_approval_task SET state = $3
        WHERE did = $1 AND approver = $2
    `
	result, err := tx.ExecContext(r.Ctx, statement, did, approver, state)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("user has no review task for this contract template")
	}
	return nil
}

func (r *PostgresApprovalTaskRepo) IsValidApprover(tx *sqlx.Tx, did string, approver string) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_approval_task
        WHERE did = $1 AND approver = $2
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, approver)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresApprovalTaskRepo) TaskExistsInState(tx *sqlx.Tx, did string, approver string, state string) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_approval_task
        WHERE did = $1 AND approver = $2 AND state = $3
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, approver, state)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresApprovalTaskRepo) TaskExists(tx *sqlx.Tx, did string) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_approval_task
        WHERE did = $1
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresApprovalTaskRepo) Delete(tx *sqlx.Tx, did string) error {
	statement := `
        DELETE FROM contract_approval_task
        WHERE did = $1
    `
	_, err := tx.ExecContext(r.Ctx, statement, did)
	return err
}
