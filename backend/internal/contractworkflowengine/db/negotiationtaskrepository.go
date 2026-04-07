package db

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type NegotiationTaskData struct {
	ID         string    `db:"id"`
	DID        string    `db:"did"`
	State      string    `db:"state"`
	Negotiator string    `db:"negotiator"`
	CreatedBy  string    `db:"created_by"`
	CreatedAt  time.Time `db:"created_at"`
}

type NegotiationTaskRepo interface {
	Create(tx *sqlx.Tx, data NegotiationTaskData) (*time.Time, error)
	IsValidNegotiator(tx *sqlx.Tx, did string, negotiator string) (bool, error)
	ReopenTasks(tx *sqlx.Tx, did string) error
	ReadAll(tx *sqlx.Tx, did string) ([]NegotiationTaskData, error)
	ReadAllByDID(tx *sqlx.Tx, did string) ([]NegotiationTaskData, error)
	ReadAllByNegotiator(tx *sqlx.Tx, reviewer string) ([]NegotiationTaskData, error)
	ReadNegotiatorsForDID(tx *sqlx.Tx, did string) ([]string, error)
	UpdateState(tx *sqlx.Tx, did string, negotiator string, state string) error
	AnyTasksInState(tx *sqlx.Tx, did string, states ...string) (bool, error)
	TaskExistsInState(tx *sqlx.Tx, did string, negotiator string, state string) (bool, error)
	TaskExist(tx *sqlx.Tx, did string) (bool, error)
	Delete(tx *sqlx.Tx, did string) error
}
