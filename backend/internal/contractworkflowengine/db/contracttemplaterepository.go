package db

import (
	"digital-contracting-service/internal/base/datatype"

	"github.com/jmoiron/sqlx"
)

type ContractTemplateRepo interface {
	ReadTemplateDataByID(tx *sqlx.Tx, did string) (*datatype.JSON, error)
}
