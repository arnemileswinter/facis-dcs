package pq

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

type PostgresAuditAndComplianceRepository struct{}

func (r *PostgresAuditAndComplianceRepository) UpdateLogCID(ctx context.Context, tx *sqlx.Tx, component string, did string, lastLogDID *string) error {
	statement := `UPDATE audit_trail_log SET last_log_cid = $2 WHERE did = $1`
	result, err := tx.ExecContext(ctx, statement, did, lastLogDID)
	rows, _ := result.RowsAffected()

	if rows == 0 {
		statement := `
        INSERT INTO audit_trail_log (
            component, did, last_log_cid
        ) VALUES ($1, $2, $3)
    	`
		_, err = tx.ExecContext(ctx, statement, component, did, lastLogDID)
		if err != nil {
			return err
		}
		return nil
	}

	return err
}

func (r *PostgresAuditAndComplianceRepository) ReadLogCID(ctx context.Context, tx *sqlx.Tx, component string, did string) (*string, error) {
	query := `
        SELECT last_log_cid
        FROM audit_trail_log WHERE did = $1 AND component = $2;
    `
	var result *string
	err := tx.GetContext(ctx, &result, query, did, component)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}
