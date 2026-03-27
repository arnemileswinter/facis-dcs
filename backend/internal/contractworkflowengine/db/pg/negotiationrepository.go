package pg

import (
	"context"
	"digital-contracting-service/internal/contractworkflowengine/db"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostgresNegotiationRepo struct {
	Ctx context.Context
}

func (r PostgresNegotiationRepo) Create(tx *sqlx.Tx, data db.NegotiationData) (*time.Time, error) {
	statement := `
        INSERT INTO contract_negotiations (
            did, contract_version, change_request, assigned_to, created_by
        ) VALUES ($1, $2, $3, $4, $5)
        RETURNING created_at
    `
	var createdAt time.Time
	err := tx.GetContext(r.Ctx, &createdAt, statement,
		data.DID, data.ContractVersion, data.ChangeRequest, data.AssignedTo, data.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &createdAt, nil
}

func (r PostgresNegotiationRepo) Accept(tx *sqlx.Tx, id int, acceptedBy string) error {
	statement := `
        UPDATE contract_negotiations
        SET
            decision = 'ACCEPTED'
        WHERE did = $1 AND decision IS NULL AND assigned_to = $3
    `
	result, err := tx.ExecContext(r.Ctx, statement, id, acceptedBy)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("no negotiations accepted")
	}

	return nil
}

func (r PostgresNegotiationRepo) Reject(tx *sqlx.Tx, id int, rejectedBy string, rejectionReason *string) error {
	statement := `
        UPDATE contract_negotiations
        SET
            decision = CASE
                WHEN did = $1 AND assigned_to = $2 THEN 'REJECTED'
                ELSE 'CLOSED'
            END,
            rejection_reason = CASE
                WHEN did = $1 AND assigned_to = $2 THEN $3
            END
        WHERE decision IS NULL
    `
	result, err := tx.ExecContext(r.Ctx, statement, id, rejectedBy, rejectionReason)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("no negotiations rejected")
	}

	return nil
}

func (r PostgresNegotiationRepo) IsValidNegotiator(tx *sqlx.Tx, did string, contractVersion *int, negotiator string) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_negotiations
        WHERE did = $1 AND assigned_to = $2 AND contract_version = $3
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, negotiator, contractVersion)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r PostgresNegotiationRepo) ReadAllByContractDID(tx *sqlx.Tx, did string) ([]db.NegotiationData, error) {
	query := `
        SELECT id, did, contract_version, change_request, assigned_to, decision,
               rejection_reason, created_by, created_at
        FROM contract_negotiations WHERE did = $1
    `
	var negotiations []db.NegotiationData
	err := tx.SelectContext(r.Ctx, &negotiations, query, did)
	if err != nil {
		return nil, err
	}
	return negotiations, nil
}

func (r PostgresNegotiationRepo) HasOpenNegotiations(tx *sqlx.Tx, did string, contractVersion *int) (bool, error) {
	query := `
        SELECT COUNT(*) FROM contract_negotiations
        WHERE id = $1 AND contract_version = $2 AND decision IS NULL
    `
	var count int
	err := tx.GetContext(r.Ctx, &count, query, did, contractVersion)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r PostgresNegotiationRepo) AllNegotiationsAccepted(tx *sqlx.Tx, did string, contractVersion *int) (bool, error) {
	statement := `
        SELECT NOT EXISTS (
            SELECT 1 FROM contract_negotiations
            WHERE did = $1 AND (decision IS NULL OR decision != 'ACCEPTED')
        )
    `
	var allAccepted bool
	err := tx.QueryRowContext(r.Ctx, statement, did).Scan(&allAccepted)
	if err != nil {
		return false, err
	}

	return allAccepted, nil
}

func (r PostgresNegotiationRepo) Delete(tx *sqlx.Tx, did string) error {
	statement := `
        DELETE FROM contract_review_task
        WHERE did = $1
    `
	_, err := tx.ExecContext(r.Ctx, statement, did)
	return err
}
