package db

import (
	"digital-contracting-service/internal/base/datatype"
	"time"

	"github.com/jmoiron/sqlx"
)

type NegotiationCreateData struct {
	DID             string         `db:"did"`
	ContractVersion *int           `db:"contract_version"`
	ChangeRequest   *datatype.JSON `db:"change_request"`
	CreatedBy       string         `db:"created_by"`
}

type NegotiationData struct {
	ID              string         `db:"id"`
	DID             string         `db:"did"`
	ContractVersion *int           `db:"contract_version"`
	ChangeRequest   *datatype.JSON `db:"change_request"`
	Counterpart     string         `db:"counterpart"`
	Decision        *string        `db:"decision"`
	RejectionReason *string        `db:"rejection_reason"`
	CreatedBy       string         `db:"created_by"`
	CreatedAt       time.Time      `db:"created_at"`
}

type NegotiationChangeData struct {
	ID            string         `db:"id"`
	ChangeRequest *datatype.JSON `db:"change_request"`
}

type NegotiationRepo interface {
	Create(tx *sqlx.Tx, data NegotiationCreateData, counterpart []string) (*time.Time, error)
	Accept(tx *sqlx.Tx, id string, acceptedBy string) error
	Reject(tx *sqlx.Tx, id string, rejectedBy string, rejectionReason *string) error
	IsValidCounterpart(tx *sqlx.Tx, did string, contractVersion *int, counterpart string) (bool, error)
	ReadAllByContractDID(tx *sqlx.Tx, did string) ([]NegotiationData, error)
	ReadAllAcceptedByContractDIDAndVersion(tx *sqlx.Tx, did string, contractVersion *int) ([]NegotiationChangeData, error)
	HasOpenNegotiationDecisions(tx *sqlx.Tx, did string, contractVersion *int) (bool, error)
	Delete(tx *sqlx.Tx, did string) error
}
