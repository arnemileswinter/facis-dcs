package qry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/approvaltaskstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/eventtype"
	cwedb "digital-contracting-service/internal/contractworkflowengine/db"
	event2 "digital-contracting-service/internal/processauditandcompliance/event"
)

// RiskTypeMissingApproval flags a contract sitting in an approval-pending
// state while at least one required approval task is still OPEN
// (DCS-FR-PACM-03: contracts failing compliance checks are flagged for
// review; the missing approval is the policy violation being monitored).
const RiskTypeMissingApproval = "MISSING_APPROVAL"

// RiskTypeUnauthorizedAccess flags a contract carrying a persisted
// access-denial audit artifact (CONTRACT_ACCESS_DENIED, committed by the
// party read-scoping denial branch in query/contract/querybyid.go before the
// 403 is returned): the denied attempt is the unauthorized-access violation
// being monitored (DCS-FR-PACM-02).
const RiskTypeUnauthorizedAccess = "UNAUTHORIZED_ACCESS"

// approvalPendingStates are the contract states in which an OPEN approval
// task means the contract is waiting on a required approval decision.
var approvalPendingStates = map[contractstate.ContractState]bool{
	contractstate.Submitted: true,
	contractstate.Reviewed:  true,
}

// accessDenial is one persisted CONTRACT_ACCESS_DENIED artifact read back
// from the outbox audit rows; RetrievedBy is the denied actor (the
// OID4VP-disclosed organization claim the read path persists).
type accessDenial struct {
	DID         string `db:"did"`
	RetrievedBy string `db:"retrieved_by"`
}

// unauthorizedAccessRisks maps persisted access denials to compliance risks,
// one per distinct (contract, actor) pair, mirroring the MISSING_APPROVAL
// per-(contract, approver) dedup.
func unauthorizedAccessRisks(denials []accessDenial, checkedAt time.Time) []event2.ComplianceRisk {
	risks := make([]event2.ComplianceRisk, 0)
	seen := make(map[string]bool)
	for _, denial := range denials {
		if denial.DID == "" || seen[denial.DID+"|"+denial.RetrievedBy] {
			continue
		}
		seen[denial.DID+"|"+denial.RetrievedBy] = true
		risks = append(risks, event2.ComplianceRisk{
			DID:      denial.DID,
			RiskType: RiskTypeUnauthorizedAccess,
			Detail: fmt.Sprintf(
				"unauthorized access to contract %s was attempted by %s and denied",
				denial.DID, denial.RetrievedBy),
			DetectedAt: checkedAt,
		})
	}
	return risks
}

type MonitorQry struct {
	MonitoredBy string
	HolderDID   string
	UserRoles   userrole.UserRoles
}

type MonitorResult struct {
	CheckedAt time.Time
	Risks     []event2.ComplianceRisk
}

type ComplianceMonitor struct {
	DB     *sqlx.DB
	ATRepo cwedb.ApprovalTaskRepo
	CRepo  cwedb.ContractRepo
}

// Handle sweeps all OPEN approval tasks and flags those whose contract is in
// an approval-pending state as MISSING_APPROVAL risks, then reads back the
// persisted access-denial artifacts and flags the affected contracts as
// UNAUTHORIZED_ACCESS risks. The sweep itself is recorded in the audit trail
// via ComplianceMonitorEvent, risks included, so a detected risk is both
// flagged (response) and reported (audit trail).
func (h *ComplianceMonitor) Handle(ctx context.Context, query MonitorQry) (*MonitorResult, error) {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	openTasks, err := h.ATRepo.ReadAllInState(ctx, tx, approvaltaskstate.Open.String())
	if err != nil {
		return nil, fmt.Errorf("could not read open approval tasks: %w", err)
	}

	checkedAt := time.Now().UTC()
	risks := make([]event2.ComplianceRisk, 0)
	seen := make(map[string]bool)
	for _, task := range openTasks {
		if seen[task.DID+"|"+task.Approver] {
			continue
		}
		seen[task.DID+"|"+task.Approver] = true

		data, err := h.CRepo.ReadDataByDID(ctx, tx, task.DID)
		if err != nil {
			return nil, fmt.Errorf("could not read contract %s for open approval task: %w", task.DID, err)
		}
		state, err := contractstate.NewContractState(data.State)
		if err != nil {
			return nil, fmt.Errorf("could not parse contract state for %s: %w", task.DID, err)
		}
		if !approvalPendingStates[state] {
			continue
		}
		risks = append(risks, event2.ComplianceRisk{
			DID:      task.DID,
			RiskType: RiskTypeMissingApproval,
			Detail: fmt.Sprintf(
				"contract in state %s is missing a required approval from %s",
				state, task.Approver),
			DetectedAt: checkedAt,
		})
	}

	denialQuery := `
        SELECT COALESCE(did, '') AS did,
               COALESCE(event_data ->> 'retrieved_by', '') AS retrieved_by
        FROM outbox_events
        WHERE event_type = $1
        ORDER BY id
    `
	var denials []accessDenial
	if err := tx.SelectContext(ctx, &denials, denialQuery, eventtype.AccessDenied.String()); err != nil {
		return nil, fmt.Errorf("could not read persisted access denials: %w", err)
	}
	risks = append(risks, unauthorizedAccessRisks(denials, checkedAt)...)

	evt := event2.ComplianceMonitorEvent{
		MonitoredBy: query.MonitoredBy,
		OccurredAt:  checkedAt,
		Risks:       risks,
		HolderDID:   query.HolderDID,
		UserRoles:   query.UserRoles,
	}
	if err := event.Create(ctx, tx, evt, componenttype.ProcessAuditAndCompliance); err != nil {
		return nil, fmt.Errorf("could not create monitor event: %w", err)
	}
	// Anchor each risk against the affected contract's PAC chain — the
	// sweep event above has no resource DID and only reaches the global
	// chain (see ComplianceRiskEvent doc).
	for _, risk := range risks {
		riskEvt := event2.ComplianceRiskEvent{
			DID:         risk.DID,
			RiskType:    risk.RiskType,
			Detail:      risk.Detail,
			MonitoredBy: query.MonitoredBy,
			OccurredAt:  checkedAt,
			HolderDID:   query.HolderDID,
			UserRoles:   query.UserRoles,
		}
		if err := event.Create(ctx, tx, riskEvt, componenttype.ProcessAuditAndCompliance); err != nil {
			return nil, fmt.Errorf("could not create compliance risk event for %s: %w", risk.DID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	return &MonitorResult{CheckedAt: checkedAt, Risks: risks}, nil
}
