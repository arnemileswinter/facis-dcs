package db

import (
	"digital-contracting-service/internal/base/datatype"
	"time"

	"github.com/jmoiron/sqlx"
)

type NegotiationData struct {
	ID int `db:"id"`

	DID             string         `db:"did"`
	ContractVersion *int           `db:"contract_version"`
	ChangeRequest   *datatype.JSON `db:"change_request"`

	AssignedTo   string  `db:"assigned_to"`
	Decision     *string `db:"decision"`
	RejectReason *string `db:"rejection_reason"`

	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}

type NegotiationRepo interface {
	Create(tx *sqlx.Tx, data NegotiationData) (*time.Time, error)
	Accept(tx *sqlx.Tx, id int, acceptedBy string) error
	Reject(tx *sqlx.Tx, id int, rejectedBy string, rejectionReason *string) error
	IsValidNegotiator(tx *sqlx.Tx, did string, contractVersion *int, negotiator string) (bool, error)
	ReadAllByContractDID(tx *sqlx.Tx, did string) ([]NegotiationData, error)
	HasOpenNegotiations(tx *sqlx.Tx, did string, contractVersion *int) (bool, error)
	AllNegotiationsAccepted(tx *sqlx.Tx, did string, contractVersion *int) (bool, error)
	Delete(tx *sqlx.Tx, did string) error
}
