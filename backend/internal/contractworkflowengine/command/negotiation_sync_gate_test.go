package command

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/contractworkflowengine/db"
)

type outboundRepoFake struct {
	pending []db.NegotiationOutboundMessage
	err     error
}

func (r *outboundRepoFake) Enqueue(context.Context, *sqlx.Tx, db.NegotiationOutboundMessage) (string, error) {
	return "", nil
}
func (r *outboundRepoFake) CancelUndeliveredProposal(context.Context, *sqlx.Tx, string, string) error {
	return nil
}
func (r *outboundRepoFake) GetUndelivered(context.Context, *sqlx.Tx, string) ([]db.NegotiationOutboundMessage, error) {
	return nil, nil
}
func (r *outboundRepoFake) GetUndeliveredForDID(context.Context, *sqlx.Tx, string, string) ([]db.NegotiationOutboundMessage, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.pending, nil
}
func (r *outboundRepoFake) MarkDelivered(context.Context, *sqlx.Tx, string) error { return nil }

func TestRequireConfirmedNegotiationDeliveryAllowsEmptyQueue(t *testing.T) {
	err := requireConfirmedNegotiationDelivery(context.Background(), nil, &outboundRepoFake{}, "did:web:a", "contract")
	if err != nil {
		t.Fatalf("empty queue must allow Accept/Submit: %v", err)
	}
}

func TestRequireConfirmedNegotiationDeliveryBlocksWhenPending(t *testing.T) {
	err := requireConfirmedNegotiationDelivery(context.Background(), nil, &outboundRepoFake{
		pending: []db.NegotiationOutboundMessage{{
			ID: "1", DID: "contract", Kind: db.NegotiationMessageProposal, RecordedAt: time.Now(),
		}},
	}, "did:web:a", "contract")
	if err != ErrNegotiationSyncPending {
		t.Fatalf("want ErrNegotiationSyncPending, got %v", err)
	}
}
