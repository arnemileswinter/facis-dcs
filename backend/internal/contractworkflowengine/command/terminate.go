package command

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/identity"

	db2 "digital-contracting-service/internal/dcstodcs/db"

	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"digital-contracting-service/internal/pdfgeneration/statuspublication"

	"github.com/jmoiron/sqlx"
)

type TerminateCmd struct {
	DID          string             `json:"did"`
	TerminatedBy string             `json:"terminated_by"`
	Reason       string             `json:"reason"`
	UpdatedAt    time.Time          `json:"updated_at"`
	HolderDID    string             `json:"holder_did"`
	UserRoles    userrole.UserRoles `json:"user_roles"`
	CauserDID    string             `json:"causer_did"`
}

type Terminator struct {
	DB          *sqlx.DB
	CRepo       db.ContractRepo
	RTRepo      db.ReviewTaskRepo
	ATRepo      db.ApprovalTaskRepo
	NRepo       db.NegotiationRepo
	NTRepo      db.NegotiationTaskRepo
	SRepo       db2.SyncRepository
	DIDDocument identity.DIDDocument
}

func (h *Terminator) Handle(ctx context.Context, cmd TerminateCmd) error {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	processData, err := h.CRepo.ReadProcessDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	localPeer, err := h.DIDDocument.GetID()
	if err != nil {
		return err
	}

	// Optimistic concurrency: reject if the caller's view of the contract is
	// older than what's stored (see package doc / ADR-0007).
	if cmd.UpdatedAt.Unix() < processData.UpdatedAt.Unix() {
		if localPeer != cmd.CauserDID {
			return fmt.Errorf("contract %w, please force synchronisation and reload", base.ErrUpdatedElsewhere)
		}
		return fmt.Errorf("contract %w, please reload", base.ErrUpdatedElsewhere)
	}

	// Terminate is allowed from any non-terminal state; there is no path back
	// out of TERMINATED. See contractstate.Transitions for the exact table.
	if err := contractstate.ValidateTransition(contractstate.ContractState(processData.State), contractstate.EventTerminate); err != nil {
		return err
	}

	err = h.CRepo.UpdateState(ctx, tx, cmd.DID, contractstate.Terminated.String())
	if err != nil {
		return fmt.Errorf("could not update contract state: %w", err)
	}

	if err := closeOpenTasks(ctx, tx, cmd.DID); err != nil {
		return err
	}

	// A local termination notifies every counterparty instance
	// (DCS-NFR-BR-06). A termination received from a peer does not: the
	// terminating instance notifies all parties itself.
	if cmd.CauserDID == localPeer {
		if err := h.queuePeerTerminations(ctx, tx, cmd.DID, cmd.Reason, localPeer); err != nil {
			return err
		}
	}

	occurredAt := time.Now().UTC()
	evt := contractevents.TerminateEvent{
		DID:             cmd.DID,
		ContractVersion: processData.ContractVersion,
		TerminatedBy:    cmd.TerminatedBy,
		Reason:          cmd.Reason,
		OccurredAt:      occurredAt,
		HolderDID:       cmd.HolderDID,
		UserRoles:       cmd.UserRoles,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}
	if err := statuspublication.EnqueueTx(ctx, tx, cmd.DID, contractstate.Terminated.String(), cmd.Reason, occurredAt); err != nil {
		return err
	}

	return tx.Commit()
}

// queuePeerTerminations records one termination notice per party instance
// other than this one; dcstodcs.TerminationNotifier delivers them.
func (h *Terminator) queuePeerTerminations(ctx context.Context, tx *sqlx.Tx, did, reason, localPeer string) error {
	contract, err := h.CRepo.ReadDataByDID(ctx, tx, did)
	if err != nil {
		return fmt.Errorf("could not read contract parties: %w", err)
	}
	if contract.Responsible == nil {
		return nil
	}
	for _, party := range contract.Responsible.GetParties() {
		if party == localPeer {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO contract_peer_terminations (did, peer_did, reason) VALUES ($1, $2, $3)
			ON CONFLICT (did, peer_did) DO NOTHING`, did, party, reason); err != nil {
			return fmt.Errorf("could not queue the termination notice toward %s: %w", party, err)
		}
	}
	return nil
}

// closeOpenTasks removes the contract's open negotiation, review and approval
// tasks: a terminated contract leaves every active workflow (DCS-FR-CWE-23).
// Decided tasks stay as the record of who decided what.
func closeOpenTasks(ctx context.Context, tx *sqlx.Tx, did string) error {
	for _, table := range []string{"contract_negotiation_task", "contract_review_task", "contract_approval_task"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE did = $1 AND state = 'OPEN'`, did); err != nil {
			return fmt.Errorf("could not close open tasks in %s: %w", table, err)
		}
	}
	return nil
}
