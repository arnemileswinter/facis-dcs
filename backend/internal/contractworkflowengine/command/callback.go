package command

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

// ErrDeploymentCallbackUnauthorized is returned when the caller is not the
// target system the deployment was dispatched to (DCS-IR-SI-05).
var ErrDeploymentCallbackUnauthorized = errors.New("caller is not the target system this deployment was dispatched to")

// ErrDeploymentNotFound is returned when a callback references a correlation
// ID that was never dispatched by Deployer.
var ErrDeploymentNotFound = errors.New("deployment correlation id not found")

// DeploymentReceiptPayload is the target's execution-evidence receipt
// carried in an acknowledgement callback.
type DeploymentReceiptPayload struct {
	CorrelationID string `json:"correlation_id"`
	PayloadHash   string `json:"payload_hash"`
	ActivatedAt   string `json:"activated_at"`
}

// DeploymentCallbackCmd carries a POST /contract/deployment/callback
// request: either an ack/status update (Status/Receipt set) or a KPI report
// (KPIMetric set), or both.
type DeploymentCallbackCmd struct {
	// CallerClientID is the OAuth2 client the request authenticated as, taken
	// from the validated access token. The callback is accepted only when it
	// matches the credential of the target the deployment went to, so a target
	// can report on its own deployments and on no one else's.
	CallerClientID string
	DID            string
	CorrelationID string
	Status        string
	Receipt       *DeploymentReceiptPayload
	KPIMetric     string
	KPIValue      string
}

// DeploymentCallbackHandler validates the shared secret, then applies an
// ack (sealing an RFC-3161-timestamped execution-evidence receipt into the
// archive entry and moving the contract SIGNED -> ACTIVE, DCS-FR-SM-10/
// DCS-FR-SM-12) and/or records a reported KPI value, flagging a violation
// when it crosses the contract's own ODRL SLA constraint (DCS-FR-CWE-09).
type DeploymentCallbackHandler struct {
	DB             *sqlx.DB
	CRepo          db.ContractRepo
	DeploymentRepo db.DeploymentRepo
	TargetRepo     db.ContractTargetRepo
	ArchiveTSA     ArchiveTimestampIssuer
}

func (h *DeploymentCallbackHandler) Handle(ctx context.Context, cmd DeploymentCallbackCmd) error {
	if strings.TrimSpace(cmd.CallerClientID) == "" {
		return ErrDeploymentCallbackUnauthorized
	}

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	deployment, err := h.DeploymentRepo.FindDeploymentByCorrelationID(ctx, tx, cmd.CorrelationID)
	if err != nil {
		return fmt.Errorf("could not read deployment %s: %w", cmd.CorrelationID, err)
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}

	// One shared secret proved only that SOME target was calling. Binding the
	// caller's own credential to the registry entry the deployment was
	// dispatched to means a target can acknowledge its own deployments and
	// nothing else, and a compromised target cannot speak for the others.
	if err := h.authorizeCaller(ctx, tx, deployment, cmd.CallerClientID); err != nil {
		return err
	}

	if cmd.Receipt != nil || strings.TrimSpace(cmd.Status) != "" {
		if err := h.applyAcknowledgement(ctx, tx, deployment, cmd); err != nil {
			return err
		}
	}

	if strings.TrimSpace(cmd.KPIMetric) != "" {
		if err := h.applyKPIReport(ctx, tx, deployment, cmd); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// authorizeCaller refuses a callback that does not come from the registry entry
// the deployment was dispatched to. A deployment with no target recorded, or a
// target with no credential issued, cannot be acknowledged at all: there is
// nothing to check the caller against, and accepting it would restore exactly
// the "some target said so" property the shared secret had.
func (h *DeploymentCallbackHandler) authorizeCaller(ctx context.Context, tx *sqlx.Tx, deployment *db.ContractDeployment, callerClientID string) error {
	if deployment.TargetID == nil || strings.TrimSpace(*deployment.TargetID) == "" {
		return ErrDeploymentCallbackUnauthorized
	}

	target, err := h.TargetRepo.ReadTarget(ctx, tx, *deployment.TargetID)
	if err != nil {
		return fmt.Errorf("could not read the target of deployment %s: %w", deployment.CorrelationID, err)
	}
	if target == nil || target.OAuthClientID == nil {
		return ErrDeploymentCallbackUnauthorized
	}
	if strings.TrimSpace(*target.OAuthClientID) != strings.TrimSpace(callerClientID) {
		return ErrDeploymentCallbackUnauthorized
	}
	return nil
}

func (h *DeploymentCallbackHandler) applyAcknowledgement(ctx context.Context, tx *sqlx.Tx, deployment *db.ContractDeployment, cmd DeploymentCallbackCmd) error {
	activatedAt := time.Now().UTC()
	receipt := DeploymentReceiptPayload{
		CorrelationID: cmd.CorrelationID,
		PayloadHash:   deployment.ContentHash,
	}
	if cmd.Receipt != nil {
		if cmd.Receipt.PayloadHash != "" {
			receipt.PayloadHash = cmd.Receipt.PayloadHash
		}
		if cmd.Receipt.ActivatedAt != "" {
			receipt.ActivatedAt = cmd.Receipt.ActivatedAt
		}
	}
	if receipt.ActivatedAt == "" {
		receipt.ActivatedAt = activatedAt.Format(time.RFC3339Nano)
	}

	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal execution-evidence receipt: %w", err)
	}
	receiptSum := sha256.Sum256(receiptBytes)
	receiptHash := "sha256:" + hex.EncodeToString(receiptSum[:])

	tsaToken := ""
	if h.ArchiveTSA != nil && h.ArchiveTSA.Enabled() {
		tsaReceipt, err := h.ArchiveTSA.TimestampBytes(ctx, receiptBytes)
		if err != nil {
			return fmt.Errorf("could not timestamp execution-evidence receipt: %w", err)
		}
		tsaToken = tsaReceipt.Token
	}

	if err := h.DeploymentRepo.AcknowledgeDeployment(ctx, tx, cmd.CorrelationID, receiptHash, tsaToken, activatedAt); err != nil {
		return fmt.Errorf("could not acknowledge deployment %s: %w", cmd.CorrelationID, err)
	}

	processData, err := h.CRepo.ReadProcessDataByDID(ctx, tx, deployment.DID)
	if err != nil {
		return fmt.Errorf("could not read contract %s: %w", deployment.DID, err)
	}
	if err := contractstate.ValidateTransition(contractstate.ContractState(processData.State), contractstate.EventDeploy); err != nil {
		return err
	}
	if err := h.CRepo.UpdateState(ctx, tx, deployment.DID, contractstate.Active.String()); err != nil {
		return fmt.Errorf("could not activate contract %s: %w", deployment.DID, err)
	}

	return nil
}

func (h *DeploymentCallbackHandler) applyKPIReport(ctx context.Context, tx *sqlx.Tx, deployment *db.ContractDeployment, cmd DeploymentCallbackCmd) error {
	contract, err := h.CRepo.ReadDataByDID(ctx, tx, deployment.DID)
	if err != nil {
		return fmt.Errorf("could not read contract %s: %w", deployment.DID, err)
	}
	violation := false
	if contract.ContractData != nil && contract.ContractData.IsNotNullValue() {
		violation, err = validation.EvaluateKPIViolation(ctx, contract.ContractData, cmd.KPIMetric, cmd.KPIValue)
		if err != nil {
			return fmt.Errorf("could not evaluate KPI %q against contract %s policies: %w", cmd.KPIMetric, deployment.DID, err)
		}
	}
	correlationID := cmd.CorrelationID

	if err := h.DeploymentRepo.CreateKPI(ctx, tx, db.ContractKPI{
		DID:           deployment.DID,
		CorrelationID: &correlationID,
		Metric:        cmd.KPIMetric,
		Value:         cmd.KPIValue,
		ObservedAt:    time.Now().UTC(),
		Violation:     violation,
	}); err != nil {
		return fmt.Errorf("could not store KPI report: %w", err)
	}

	return nil
}
