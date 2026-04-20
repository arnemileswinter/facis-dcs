package pg

import (
	"context"
	"database/sql"
	"digital-contracting-service/internal/base/datatype"

	"github.com/jmoiron/sqlx"
)

type PostgresContractTemplateRepo struct {
}

func (r *PostgresContractTemplateRepo) ReadTemplateDataByID(ctx context.Context, tx *sqlx.Tx, did string) (*datatype.JSON, error) {
	statement := `
        SELECT template_data
        FROM contract_templates ct
        WHERE ct.did = $1 AND EXISTS (
        		SELECT 1
        		FROM contract_templates_approval_task ctat
        		WHERE
        			ctat.did = ct.did AND
        			ctat.state = 'APPROVED'
        )
        LIMIT 1
		`

	var templateData datatype.JSON
	if err := tx.GetContext(ctx, &templateData, statement, did); err != nil {
		return nil, err
	}
	if !templateData.IsNotNullValue() {
		return nil, sql.ErrNoRows
	}
	return &templateData, nil
}
