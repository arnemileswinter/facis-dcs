// Package contractworkflowengine implements the core contract lifecycle:
// creation, negotiation, review, approval, termination, expiry and
// renewal-deadline notices (this file's CronJob). It follows the
// repository-wide CQRS layout (command/, query/, db/, datatype/, event/),
// plus two contract-specific additions: negotiationmerging (folds accepted
// change requests into a new contract version) and remotesync (the
// command/RPC side of DCS-to-DCS federation; the peer-transport side lives
// in the separate dcstodcs package).
//
// Ownership model: every contract has a single writer, its Origin peer (see
// command package doc). Expiry is additionally exposed instantly to readers
// via the contracts_effective DB view (EXPIRED computed at query time),
// while this file's cron job lags behind to persist the state and emit the
// corresponding event/audit-trail entry.
package contractworkflowengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/conf"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/expirationpolicy"
	database "digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
)

type CronJob struct {
	DB    *sqlx.DB
	CRepo database.ContractRepo
}

func (j CronJob) Start(ctx context.Context, db *sqlx.DB) {
	go startExpiryScheduler(ctx, db, j.CRepo, conf.ExpirationCronJobTimeOut())
}

func startExpiryScheduler(ctx context.Context, db *sqlx.DB, repo database.ContractRepo, interval time.Duration) {

	readContracts := func(read func(context.Context, *sqlx.Tx) ([]database.ContractMetadata, error)) ([]database.ContractMetadata, error) {
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf("could not rollback transaction: %v", err)
			}
		}(tx)

		contracts, err := read(ctx, tx)
		if err != nil {
			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("could not commit transaction: %w", err)
		}

		return contracts, nil
	}

	callExpirationLogic := func(expiredContract database.ContractMetadata) error {

		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf("could not rollback transaction: %v", err)
			}
		}(tx)

		var policy *expirationpolicy.ExpirationPolicy
		if expiredContract.ExpPolicy != nil {
			p, err := expirationpolicy.NewExpirationPolicy(*expiredContract.ExpPolicy)
			if err != nil {
				return fmt.Errorf("could not create expiration policy: %w", err)
			}
			policy = &p
		} else {
			return fmt.Errorf("unknown expiration policy for expired contract with DID %s", expiredContract.DID)
		}

		// Readers already see EXPIRED instantly via the contracts_effective view once
		// exp_date has passed; this persists that state physically and emits the
		// ContractExpired event, so it necessarily lags the view by up to the poll interval.
		err = repo.UpdateState(ctx, tx, expiredContract.DID, contractstate.Expired.String())
		if err != nil {
			return fmt.Errorf("could not update expired contract with DID %s: %w", expiredContract.DID, err)
		}

		state, err := contractstate.NewContractState(expiredContract.State)
		if err != nil {
			return fmt.Errorf("could not create state for expired contract with DID %s: %w", expiredContract.DID, err)
		}

		evt := contractevents.ContractExpired{
			DID:             expiredContract.DID,
			ContractVersion: expiredContract.ContractVersion,
			State:           state,
			ExpPolicy:       policy,
			OccurredAt:      time.Now().UTC(),
		}
		err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
		if err != nil {
			return fmt.Errorf("could not create event: %w", err)
		}

		switch *policy {
		case expirationpolicy.Renewal:
			fmt.Printf("ToDo: call renewal logic for expired contract with DID %s\n", expiredContract.DID)
		case expirationpolicy.Archiving:
			fmt.Printf("ToDo: call archiving logic for expired contract with DID %s\n", expiredContract.DID)
		case expirationpolicy.Termination:
			fmt.Printf("ToDo: call termination logic for expired contract with DID %s\n", expiredContract.DID)
		}

		return tx.Commit()
	}

	// The ledger claim and the event share one transaction: a failed event
	// write releases the claim, so the next tick retries the notice.
	callRenewalDueLogic := func(c database.ContractMetadata) error {
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf("could not rollback transaction: %v", err)
			}
		}(tx)

		evt, err := newRenewalDueEvent(c, time.Now().UTC())
		if err != nil {
			return err
		}
		claimed, err := repo.ClaimRenewalNotice(ctx, tx, evt.DID, evt.ExpDate)
		if err != nil {
			return fmt.Errorf("could not record renewal notice for contract with DID %s: %w", c.DID, err)
		}
		if !claimed {
			return nil
		}
		if err := event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine); err != nil {
			return fmt.Errorf("could not create event: %w", err)
		}
		return tx.Commit()
	}

	ticker := time.NewTicker(interval)
	for range ticker.C {

		expiredContracts, err := readContracts(repo.ReadExpiredContracts)
		if err != nil {
			log.Printf("could not read expired contracts: %v", err)
			continue
		}

		if len(expiredContracts) > 0 {
			log.Printf("%d contracts expired", len(expiredContracts))
		}

		for _, expiredContract := range expiredContracts {
			err = callExpirationLogic(expiredContract)
			if err != nil {
				log.Printf("could not call expiration logic for expired contract: %v", err)
			}
		}

		renewalDue, err := readContracts(repo.ReadRenewalDueContracts)
		if err != nil {
			log.Printf("could not read renewal-due contracts: %v", err)
			continue
		}
		for _, c := range renewalDue {
			if err := callRenewalDueLogic(c); err != nil {
				log.Printf("could not notify renewal due: %v", err)
			}
		}
	}
}

func newRenewalDueEvent(c database.ContractMetadata, now time.Time) (contractevents.ContractRenewalDue, error) {
	if c.ExpDate == nil || c.ExpNoticePeriod == nil {
		return contractevents.ContractRenewalDue{}, fmt.Errorf("renewal-due contract with DID %s has no expiration date or notice period", c.DID)
	}
	state, err := contractstate.NewContractState(c.State)
	if err != nil {
		return contractevents.ContractRenewalDue{}, fmt.Errorf("could not create state for contract with DID %s: %w", c.DID, err)
	}
	var policy *expirationpolicy.ExpirationPolicy
	if c.ExpPolicy != nil {
		p, err := expirationpolicy.NewExpirationPolicy(*c.ExpPolicy)
		if err != nil {
			return contractevents.ContractRenewalDue{}, fmt.Errorf("could not create expiration policy: %w", err)
		}
		policy = &p
	}
	return contractevents.ContractRenewalDue{
		DID:             c.DID,
		ContractVersion: c.ContractVersion,
		State:           state,
		ExpDate:         *c.ExpDate,
		ExpNoticePeriod: *c.ExpNoticePeriod,
		ExpPolicy:       policy,
		Responsible:     c.Responsible,
		OccurredAt:      now,
	}, nil
}
