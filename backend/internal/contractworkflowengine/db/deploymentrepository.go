package db

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// ContractDeployment is one dispatch of a contract to the configured
// Contract Target System (UC-05-01), keyed by CorrelationID for matching the
// target's later ack/status/KPI callbacks.
type ContractDeployment struct {
	ID              int64      `db:"id"`
	DID             string     `db:"did"`
	ContractVersion int        `db:"contract_version"`
	CorrelationID   string     `db:"correlation_id"`
	ContentHash     string     `db:"content_hash"`
	TargetID        *string    `db:"target_id"`
	TargetURL       *string    `db:"target_url"`
	Status          string     `db:"status"`
	RequestedBy     string     `db:"requested_by"`
	RequestedAt     time.Time  `db:"requested_at"`
	AcknowledgedAt  *time.Time `db:"acknowledged_at"`
	ReceiptHash     *string    `db:"receipt_hash"`
	TSAToken        *string    `db:"tsa_token"`
	DispatchError   *string    `db:"dispatch_error"`
}

// ContractKPI is a single KPI value reported via the deployment callback for
// an ACTIVE contract (DCS-FR-CWE-09, DCS-FR-CWE-31).
type ContractKPI struct {
	ID            int64     `db:"id"`
	DID           string    `db:"did"`
	CorrelationID *string   `db:"correlation_id"`
	Metric        string    `db:"metric"`
	Value         string    `db:"value"`
	ObservedAt    time.Time `db:"observed_at"`
	Violation     bool      `db:"violation"`
}

// ContractTarget is one configured Contract Target System deployments may be
// dispatched to (ADR-25). It replaces the single CONTRACT_TARGET_URL: a DCS
// serving several execution environments could not otherwise say where a
// contract should go, and a failed dispatch had no target to name.
type ContractTarget struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	URL         string    `db:"url"`
	Description *string   `db:"description"`
	Enabled     bool      `db:"enabled"`
	CreatedBy   string    `db:"created_by"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type ContractTargetRepo interface {
	ListTargets(ctx context.Context, tx *sqlx.Tx) ([]ContractTarget, error)
	ReadTarget(ctx context.Context, tx *sqlx.Tx, id string) (*ContractTarget, error)
	CreateTarget(ctx context.Context, tx *sqlx.Tx, data ContractTarget) (*ContractTarget, error)
	UpdateTarget(ctx context.Context, tx *sqlx.Tx, data ContractTarget) (*ContractTarget, error)
	DeleteTarget(ctx context.Context, tx *sqlx.Tx, id string) error
	// CountContractsDesignating reports how many contracts still name this
	// target, so removing it can be refused rather than leaving them
	// undeployable with no record of where they were meant to go.
	CountContractsDesignating(ctx context.Context, tx *sqlx.Tx, id string) (int, error)
	// DesignateForContract sets (or with a nil targetID clears) the target a
	// contract deploys to. Staleness is checked by the caller against the
	// stored updated_at, the same second-granularity comparison every other
	// contract mutation uses.
	DesignateForContract(ctx context.Context, tx *sqlx.Tx, did string, targetID *string) (bool, error)
}

type DeploymentRepo interface {
	CreateDeployment(ctx context.Context, tx *sqlx.Tx, data ContractDeployment) error
	FindDeploymentByCorrelationID(ctx context.Context, tx *sqlx.Tx, correlationID string) (*ContractDeployment, error)
	AcknowledgeDeployment(ctx context.Context, tx *sqlx.Tx, correlationID string, receiptHash string, tsaToken string, acknowledgedAt time.Time) error
	// MarkDispatchFailed records that the outbound call never reached the
	// target. Without it the row keeps the 'DISPATCHED' status it was written
	// with before the call was attempted, making a deployment the target never
	// received indistinguishable from one it acknowledged.
	MarkDispatchFailed(ctx context.Context, tx *sqlx.Tx, correlationID string, reason string) error
	CreateKPI(ctx context.Context, tx *sqlx.Tx, data ContractKPI) error
	ReadKPIsByDID(ctx context.Context, tx *sqlx.Tx, did string) ([]ContractKPI, error)
}
