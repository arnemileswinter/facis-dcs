package pg

import (
	"context"
	"digital-contracting-service/internal/contractworkflowengine/db"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostgresNegotiationTaskRepo struct {
	Ctx context.Context
}

func (r *PostgresNegotiationTaskRepo) Create(tx *sqlx.Tx, data db.NegotiationTaskData) (*time.Time, error) {
	statement := `
        INSERT INTO contract_negotiation_task (
            did, state, negotiator, created_by
        ) VALUES ($1, $2, $3, $4)
        RETURNING created_at
    `
	var createdAt time.Time
	err := tx.GetContext(r.Ctx, &createdAt, statement,
		data.DID, data.State, data.Negotiator, data.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &createdAt, nil
}

func (r *PostgresNegotiationTaskRepo) IsValidNegotiator(tx *sqlx.Tx, did string, negotiator string) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_negotiation_task
        WHERE did = $1 AND negotiator = $2
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, negotiator)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresNegotiationTaskRepo) ReopenTasks(tx *sqlx.Tx, did string) error {
	statement := `
        UPDATE contract_negotiation_task SET state = 'OPEN'
        WHERE did = $1
    `
	_, err := tx.ExecContext(r.Ctx, statement, did)
	return err
}

func (r *PostgresNegotiationTaskRepo) ReadAll(tx *sqlx.Tx, did string) ([]db.NegotiationTaskData, error) {
	query := `
        SELECT id, did, state, negotiator,
               created_by, created_at
        FROM contract_negotiation_task WHERE did = $1
    `
	var negotiationTasks []db.NegotiationTaskData
	err := tx.SelectContext(r.Ctx, &negotiationTasks, query, did)
	if err != nil {
		return nil, err
	}
	return negotiationTasks, nil
}

func (r *PostgresNegotiationTaskRepo) ReadAllByDID(tx *sqlx.Tx, did string) ([]db.NegotiationTaskData, error) {
	query := `
        SELECT id, did, state, negotiator,
               created_by, created_at
        FROM contract_negotiation_task WHERE did = $1
    `
	var negotiationTasks []db.NegotiationTaskData
	err := tx.SelectContext(r.Ctx, &negotiationTasks, query, did)
	if err != nil {
		return nil, err
	}
	return negotiationTasks, nil
}

func (r *PostgresNegotiationTaskRepo) ReadAllByNegotiator(tx *sqlx.Tx, negotiator string) ([]db.NegotiationTaskData, error) {
	query := `
        SELECT id, did, state, negotiator,
               created_by, created_at
        FROM contract_negotiation_task WHERE negotiator = $1
    `
	var negotiationTasks []db.NegotiationTaskData
	err := tx.SelectContext(r.Ctx, &negotiationTasks, query, negotiator)
	if err != nil {
		return nil, err
	}
	return negotiationTasks, nil
}

func (r *PostgresNegotiationTaskRepo) ReadNegotiatorsForDID(tx *sqlx.Tx, did string) ([]string, error) {
	query := `
        SELECT negotiator
        FROM contract_negotiation_task WHERE did = $1
    `
	var reviewers []string
	err := tx.SelectContext(r.Ctx, &reviewers, query, did)
	if err != nil {
		return nil, err
	}
	return reviewers, nil
}

func (r *PostgresNegotiationTaskRepo) UpdateState(tx *sqlx.Tx, did string, negotiator string, state string) error {
	statement := `
        UPDATE contract_negotiation_task SET state = $3
        WHERE did = $1 AND negotiator = $2
    `
	result, err := tx.ExecContext(r.Ctx, statement, did, negotiator, state)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("user has no negotiation task for this contract")
	}
	return nil
}

func (r *PostgresNegotiationTaskRepo) AnyTasksInState(tx *sqlx.Tx, did string, states ...string) (bool, error) {
	placeholders := make([]string, len(states))
	args := []interface{}{did}

	for i, s := range states {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, s)
	}

	query := fmt.Sprintf(`
        SELECT COUNT(*) 
        FROM contract_review_task 
        WHERE did = $1 AND state IN (%s)
    `, strings.Join(placeholders, ", "))

	var count int
	err := tx.GetContext(r.Ctx, &count, query, args...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresNegotiationTaskRepo) TaskExistsInState(tx *sqlx.Tx, did string, negotiator string, state string) (bool, error) {
	query := `
        SELECT COUNT(*) 
        FROM contract_negotiation_task 
        WHERE did = $1 AND negotiator = $2 AND state = $3
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, negotiator, state)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresNegotiationTaskRepo) TaskExist(tx *sqlx.Tx, did string) (bool, error) {
	query := `
        SELECT COUNT(*) 
        FROM contract_negotiation_task 
        WHERE did = $1
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresNegotiationTaskRepo) Delete(tx *sqlx.Tx, did string) error {
	statement := `
        DELETE FROM contract_negotiation_task
        WHERE did = $1
    `
	_, err := tx.ExecContext(r.Ctx, statement, did)
	return err
}
