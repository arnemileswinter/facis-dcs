package db

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type ApprovalTaskData struct {
	ID        string    `db:"id"`
	DID       string    `db:"did"`
	State     string    `db:"state"`
	Approver  string    `db:"approver"`
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}

type ApprovalTaskRepo interface {
	Create(tx *sqlx.Tx, data ApprovalTaskData) (*time.Time, error)
	ReopenTasks(tx *sqlx.Tx, did string) error
	ReadAll(dtx *sqlx.Tx, id string) ([]ApprovalTaskData, error)
	ReadAllByApprover(tx *sqlx.Tx, approver string) ([]ApprovalTaskData, error)
	UpdateState(tx *sqlx.Tx, did string, approver string, state string) error
	IsValidApprover(tx *sqlx.Tx, did string, approver string) (bool, error)
	TaskExistsInState(tx *sqlx.Tx, did string, approver string, state string) (bool, error)
	TaskExists(tx *sqlx.Tx, did string) (bool, error)
	Delete(tx *sqlx.Tx, did string) error
}
