package db

import (
	"digital-contracting-service/internal/base/datatype"
	"time"

	"github.com/jmoiron/sqlx"
)

type NegotiationData struct {
	ID              string         `db:"id"`
	DID             string         `db:"did"`
	ContractVersion *int           `db:"contract_version"`
	ChangeRequest   *datatype.JSON `db:"change_request"`
	CreatedBy       string         `db:"created_by"`
	CreatedAt       time.Time      `db:"created_at"`
}

type NegotiationDecisionData struct {
	ID              string  `db:"id"`
	NegotiationID   string  `db:"negotiation_id"`
	AssignedTo      string  `db:"assigned_to"`
	Decision        *string `db:"decision"`
	RejectionReason *string `db:"rejection_reason"`
}

type NegotiationRepo interface {
	Create(tx *sqlx.Tx, data NegotiationData, counterpart []string) (*time.Time, error)
	Accept(tx *sqlx.Tx, id string, acceptedBy string) error
	Reject(tx *sqlx.Tx, id string, rejectedBy string, rejectionReason *string) error
	IsValidCounterpart(tx *sqlx.Tx, did string, contractVersion *int, counterpart string) (bool, error)
	ReadAllByContractDID(tx *sqlx.Tx, did string) ([]NegotiationData, error)
	HasOpenNegotiations(tx *sqlx.Tx, did string, contractVersion *int) (bool, error)
	AllNegotiationsAccepted(tx *sqlx.Tx, did string, contractVersion *int) (bool, error)
	Delete(tx *sqlx.Tx, did string) error
}
