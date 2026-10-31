package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// RunExclusive runs fn only when no other replica of this DCS instance runs
// the job called name at the same time. It reports false, and does not run
// fn, when another replica holds the job. The lock is a transaction-level
// advisory lock in a transaction that stays open while fn runs, so Postgres
// releases it when fn returns or when the replica loses its connection.
func RunExclusive(ctx context.Context, database *sqlx.DB, name string, fn func() error) (bool, error) {
	tx, err := database.BeginTxx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("lock job %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	var locked bool
	if err := tx.GetContext(ctx, &locked, `SELECT pg_try_advisory_xact_lock(hashtext('dcs-job:' || $1))`, name); err != nil {
		return false, fmt.Errorf("lock job %s: %w", name, err)
	}
	if !locked {
		return false, nil
	}
	return true, fn()
}
