package db

import (
	"context"
	"digital-contracting-service/internal/base/datatype"

	"github.com/jmoiron/sqlx"
)

type ContractTemplateRepo interface {
	ReadTemplateDataByID(ctx context.Context, tx *sqlx.Tx, did string) (*datatype.JSON, error)
}
