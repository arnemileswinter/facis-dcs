package db

import (
	"digital-contracting-service/internal/base/datatype"
	"time"

	"github.com/jmoiron/sqlx"
)

type ContractTemplate struct {
	DID            string         `db:"did"`
	DocumentNumber *string        `db:"document_number"`
	Version        *int           `db:"version"`
	State          string         `db:"state"`
	TemplateType   string         `db:"template_type"`
	Name           *string        `db:"name"`
	Description    *string        `db:"description"`
	CreatedBy      string         `db:"created_by"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	TemplateData   *datatype.JSON `db:"template_data"`
}

type ContractTemplateMetadata struct {
	DID            string    `db:"did"`
	DocumentNumber *string   `db:"document_number"`
	Version        *int      `db:"version"`
	State          string    `db:"state"`
	TemplateType   string    `db:"template_type"`
	Name           *string   `db:"name"`
	Description    *string   `db:"description"`
	CreatedBy      string    `db:"created_by"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type ContractTemplateProcessData struct {
	DID            string    `db:"did"`
	DocumentNumber *string   `db:"document_number"`
	Version        *int      `db:"version"`
	State          string    `db:"state"`
	CreatedBy      string    `db:"created_by"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type ContractTemplateUpdateData struct {
	DID            string         `db:"did"`
	DocumentNumber *string        `db:"document_number"`
	Version        *int           `db:"version"`
	State          string         `db:"state"`
	TemplateType   string         `db:"template_type"`
	Name           *string        `db:"name"`
	Description    *string        `db:"description"`
	TemplateData   *datatype.JSON `db:"template_data"`
}

type SearchValues struct {
	DID            *string
	DocumentNumber *string
	Version        *int
	State          string
	TemplateType   string
	Name           *string
	Description    *string
	Filter         *string
}

type ContractTemplateRepo interface {
	Create(tx *sqlx.Tx, data ContractTemplate) (*time.Time, error)
	ReadDataByID(tx *sqlx.Tx, did string) (*ContractTemplate, error)
	ReadAllMetaData(tx *sqlx.Tx) ([]ContractTemplateMetadata, error)
	ReadAllMetaDataByFilter(tx *sqlx.Tx, values SearchValues) ([]ContractTemplateMetadata, error)
	ReadProcessData(tx *sqlx.Tx, did string) (*ContractTemplateProcessData, error)
	UpdateState(tx *sqlx.Tx, did string, state string) error
	Update(tx *sqlx.Tx, data ContractTemplateUpdateData) error
}
