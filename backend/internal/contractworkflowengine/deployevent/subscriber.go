// Package deployevent auto-deploys a contract once its signing workflow
// completes (DCS-FR-CWE-06): it reads the signingmanagement
// APPLIED_SIGNATURE events from the outbox and calls the same
// command.Deployer the manual POST /contract/deploy endpoint uses, so both
// paths share one deployment implementation.
package deployevent

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/processauditandcompliance/workflowgate"
	smeventtype "digital-contracting-service/internal/signingmanagement/datatype/eventtype"
)

// Subscriber listens for signature-applied events and dispatches an
// automatic deployment for the signed contract.
type Subscriber struct {
	Deployer ContractDeployer
	Gate     *workflowgate.Coordinator
	// LocalPeer is this instance's own did:web, passed on so the multi-signer
	// gate reads the same on this path as on the manual endpoint: without it
	// every declared field looks local, and a federated contract's counterparty
	// slot is demanded from a database that will never hold it.
	LocalPeer string
}

// ContractDeployer is the deployment implementation this subscriber shares with
// the manual POST /contract/deploy endpoint (command.Deployer).
type ContractDeployer interface {
	Deployable(ctx context.Context, did, localPeer string) error
	Handle(ctx context.Context, cmd command.DeployCmd) (*command.DeployResult, error)
}

// OutboxConsumer hands every signature-applied event to the subscriber, read
// from the outbox table (event.OutboxConsumer), so a signature applied while no
// replica listens on NATS still deploys the contract (DCS-NFR-SF-01).
func (s *Subscriber) OutboxConsumer(database *sqlx.DB) *event.OutboxConsumer {
	return &event.OutboxConsumer{
		DB:    database,
		Name:  "auto-deploy",
		Types: []string{smeventtype.Applied.String()},
		Handle: func(ctx context.Context, _ string, data json.RawMessage) {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			// A contract that designates no target system is not an error: not every
			// party deploys what it signs — a negotiating peer may simply hold the
			// agreement. Deployment is opt-in, so this is an ordinary outcome
			// (ADR-25) and must not be reported as a failure.
			// A partially signed contract fires this event after each signature but
			// the last; that is the expected path, not a failure.
			if err := s.handle(ctx, data); err != nil && !errors.Is(err, command.ErrNoTargetDesignated) && !errors.Is(err, command.ErrSigningIncomplete) {
				log.Printf("contractworkflowengine/deployevent: could not auto-deploy: %v", err)
			}
		},
	}
}

func (s *Subscriber) handle(ctx context.Context, data json.RawMessage) error {
	var envelope struct {
		DID string `json:"did"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.DID == "" {
		return nil
	}
	// The gate is dispatched only for a contract that may deploy now.
	if err := s.Deployer.Deployable(ctx, envelope.DID, s.LocalPeer); err != nil {
		return err
	}
	if s.Gate != nil {
		if _, _, err := s.Gate.Execute(ctx, workflowgate.Input{
			Gate: "deployment", ContractDID: envelope.DID,
			Requester:    "system:auto-deploy",
			Continuation: map[string]any{"requested_by": "system:auto-deploy"},
		}); err != nil {
			return err
		}
	}

	_, err := s.Deployer.Handle(ctx, command.DeployCmd{
		DID:         envelope.DID,
		RequestedBy: "system:auto-deploy",
		LocalPeer:   s.LocalPeer,
	})
	return err
}
