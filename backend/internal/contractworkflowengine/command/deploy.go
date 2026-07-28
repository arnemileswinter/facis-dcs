package command

import (
	"bytes"
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

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

// ErrSigningIncomplete rejects deployment of a multi-signer contract whose
// declared signature fields are not all signed yet (DCS-FR-SM-07/-17).
var ErrSigningIncomplete = errors.New("signing workflow incomplete")

// DeployCmd carries the inputs for deploying a SIGNED contract to the
// configured Contract Target System (UC-05-01).
// Deployment refusals that are the operator's to fix, not internal errors
// (ADR-25). A contract that reaches signing with no destination does not deploy,
// and saying so is the point — silently doing nothing is what this replaced.
var (
	ErrNoTargetDesignated  = errors.New("this contract designates no target system to deploy to")
	ErrTargetNotRegistered = errors.New("no such target system is registered")
	ErrTargetDisabled      = errors.New("target system is disabled and cannot receive deployments")
)

type DeployCmd struct {
	DID         string
	UpdatedAt   time.Time
	RequestedBy string
	// LocalPeer is this instance's own did:web. Signature slots are named by
	// the signing party, so it identifies which declared slot is ours.
	LocalPeer string
	// TargetIDOverride re-dispatches to a different registered target than the
	// one the contract designates (ADR-25). The designation itself is unchanged:
	// this is the operator directing one delivery, not editing the contract.
	TargetIDOverride string
}

// DeployResult is what both the manual /contract/deploy endpoint and the
// automatic post-signing subscriber receive back from Deployer.Handle.
type DeployResult struct {
	DID             string
	ContractVersion int
	ContentHash     string
	Timestamp       time.Time
	CorrelationID   string
	Payload         map[string]any
	TargetID        string
	TargetName      string
}

// Deployer handles DeployCmd: it gates on the contract being SIGNED (the
// same EventDeploy edge the ack-driven SIGNED -> ACTIVE transition uses,
// declared once in contractstate.Transitions), builds the machine-readable
// deployment payload (JSON-LD contract document, DID, version, content hash,
// timestamp, and an enclosing odrl:Set describing the deployment's own
// authorization), records the dispatch, and best-effort forwards it to the
// configured Contract Target System.
type Deployer struct {
	DB             *sqlx.DB
	CRepo          db.ContractRepo
	DeploymentRepo db.DeploymentRepo
	TargetRepo     db.ContractTargetRepo
	Target         ContractTargetClient
}

func (h *Deployer) Handle(ctx context.Context, cmd DeployCmd) (*DeployResult, error) {
	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	data, err := h.CRepo.ReadDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return nil, fmt.Errorf("could not read contract %s: %w", cmd.DID, err)
	}

	if err := contractstate.ValidateTransition(contractstate.ContractState(data.State), contractstate.EventDeploy); err != nil {
		return nil, err
	}

	// Multi-signer gate (DCS-FR-SM-07/-17, DCS-NFR-BR-03): a contract that
	// declares signature fields may only deploy once EVERY declared field is
	// signed. The auto-deploy subscriber fires after each signature, so a
	// partially signed contract hits this gate until the last signatory
	// signs.
	if data.ContractData != nil && data.ContractData.IsNotNullValue() {
		required := validation.RequiredSignatureFields([]byte(*data.ContractData))
		if len(required) > 0 {
			signedFields, err := h.CRepo.ReadSignedSignatureFieldNames(ctx, tx, cmd.DID)
			if err != nil {
				return nil, fmt.Errorf("could not read signed signature fields: %w", err)
			}
			signed := make(map[string]bool, len(signedFields))
			for _, f := range signedFields {
				signed[f] = true
			}
			var missing []string
			for _, f := range required {
				if signed[f] {
					continue
				}
				// A counterparty signs in ITS deployment and its signature record
				// never reaches ours, so requiring it here means a federated
				// contract can never be activated on either side. Activation is a
				// local act on a local copy: this instance gates on its OWN slot,
				// while the peer's signature is carried by the artifact we
				// received and content-verified (the fatal /verify/content gate).
				// Fields that are not a party DID — the single-instance
				// multi-signer flow names them per signatory — are never skipped.
				if isRemotePartyField(data.Responsible, cmd.LocalPeer, f) {
					continue
				}
				missing = append(missing, f)
			}
			if len(missing) > 0 {
				return nil, fmt.Errorf("%w: unsigned signature fields: %s", ErrSigningIncomplete, strings.Join(missing, ", "))
			}
		}
	}

	contractDataBytes := []byte(`{}`)
	if data.ContractData != nil && data.ContractData.IsNotNullValue() {
		contractDataBytes = []byte(*data.ContractData)
	}
	// Decode the contract document instead of embedding the raw jsonb bytes:
	// the content hash must be reproducible by the RECEIVING target system
	// from the parsed JSON (canonical form = recursively key-sorted, compact,
	// no HTML escaping — see hashDeploymentPayload), and Postgres jsonb's
	// length-then-bytewise key order would otherwise leak into the hash.
	var contractDocument map[string]any
	if err := json.Unmarshal(contractDataBytes, &contractDocument); err != nil {
		return nil, fmt.Errorf("could not decode contract document for %s: %w", cmd.DID, err)
	}

	correlationID := uuid.NewString()
	now := time.Now().UTC()

	payload := map[string]any{
		"@context": map[string]string{
			"dcs":  "https://w3id.org/facis/dcs/ontology/v1#",
			"odrl": "http://www.w3.org/ns/odrl/2/",
		},
		"@type":                "dcs:ContractDeployment",
		"dcs:contractDid":      cmd.DID,
		"dcs:contractVersion":  data.ContractVersion,
		"dcs:timestamp":        now.Format(time.RFC3339Nano),
		"dcs:correlationId":    correlationID,
		"dcs:contractDocument": contractDocument,
		"odrl:policy": map[string]any{
			"@id":   "urn:uuid:deployment-policy-" + correlationID,
			"@type": "odrl:Set",
		},
	}

	contentHash, err := hashDeploymentPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("could not hash deployment payload: %w", err)
	}
	payload["dcs:contentHash"] = contentHash

	// Where this contract goes is the contract's own property (ADR-25), so the
	// automatic trigger on signing completion has a destination without a human
	// present to choose one. A re-dispatch may be directed elsewhere.
	targetID := strings.TrimSpace(cmd.TargetIDOverride)
	if targetID == "" && data.TargetID != nil {
		targetID = strings.TrimSpace(*data.TargetID)
	}
	if targetID == "" {
		return nil, ErrNoTargetDesignated
	}
	target, err := h.TargetRepo.ReadTarget(ctx, tx, targetID)
	if err != nil {
		return nil, fmt.Errorf("could not read target system %s: %w", targetID, err)
	}
	if target == nil {
		return nil, fmt.Errorf("%w: %s", ErrTargetNotRegistered, targetID)
	}
	if !target.Enabled {
		return nil, fmt.Errorf("%w: %s", ErrTargetDisabled, target.Name)
	}
	// The endpoint is copied as it stands now, beside the reference: editing the
	// registry entry later must not rewrite what this deployment actually did.
	targetURL := target.URL
	targetURLPtr := &targetURL
	targetIDPtr := &target.ID

	if err := h.DeploymentRepo.CreateDeployment(ctx, tx, db.ContractDeployment{
		DID:             cmd.DID,
		ContractVersion: data.ContractVersion,
		CorrelationID:   correlationID,
		ContentHash:     contentHash,
		TargetID:        targetIDPtr,
		TargetURL:       targetURLPtr,
		Status:          "DISPATCHED",
		RequestedBy:     cmd.RequestedBy,
		RequestedAt:     now,
	}); err != nil {
		return nil, fmt.Errorf("could not store deployment record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	// The target's own callback (POST /contract/deployment/callback) remains the
	// authoritative signal of a successful deployment, so a failed outbound call
	// is not fatal to the request. It is recorded, though: the row was written
	// DISPATCHED before this ran, and leaving it that way made a deployment the
	// target never received indistinguishable from one it acknowledged. The
	// compliance monitor reads these back and alerts on them (DCS-FR-CWE-31).
	if h.Target != nil {
		if err := h.Target.DeployTo(ctx, targetURL, payload); err != nil {
			log.Printf("contractworkflowengine: could not dispatch deployment %s for contract %s to %s: %v", correlationID, cmd.DID, targetURL, err)
			h.recordDispatchFailure(ctx, correlationID, err)
		}
	}

	return &DeployResult{
		DID:             cmd.DID,
		ContractVersion: data.ContractVersion,
		ContentHash:     contentHash,
		Timestamp:       now,
		CorrelationID:   correlationID,
		Payload:         payload,
		TargetID:        target.ID,
		TargetName:      target.Name,
	}, nil
}

// recordDispatchFailure marks the deployment row so the failure survives the
// process log. Its own failure is logged and swallowed: the deployment already
// happened as far as the caller is concerned, and losing the request over a
// bookkeeping error would be worse than losing the alert.
func (h *Deployer) recordDispatchFailure(ctx context.Context, correlationID string, cause error) {
	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		log.Printf("contractworkflowengine: could not record dispatch failure for %s: %v", correlationID, err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	if err := h.DeploymentRepo.MarkDispatchFailed(ctx, tx, correlationID, cause.Error()); err != nil {
		log.Printf("contractworkflowengine: could not record dispatch failure for %s: %v", correlationID, err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("contractworkflowengine: could not record dispatch failure for %s: %v", correlationID, err)
	}
}

// hashDeploymentPayload computes the payload's canonical content hash:
// recursively key-sorted (Go marshals maps sorted), compact, WITHOUT HTML
// escaping — so a receiving target system can reproduce it from the parsed
// JSON with a plain deep-sort + stringify (the shipped ORCE
// contract-target-flow and the BDD harness both do exactly that).
func hashDeploymentPayload(payload map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return "", err
	}
	canonical := bytes.TrimRight(buf.Bytes(), "\n")
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// isRemotePartyField reports whether a declared signature field belongs to a
// party other than this instance. Slots are named by the signing party's DID, so
// a slot naming the counterparty is one whose signature is produced, recorded
// and activated in the peer's own deployment. A field that is not a party DID is
// never remote.
func isRemotePartyField(resp *db.Responsible, localPeer, field string) bool {
	if resp == nil || localPeer == "" || field == "" || field == localPeer {
		return false
	}
	return field == resp.Creator || field == resp.Counterparty
}
