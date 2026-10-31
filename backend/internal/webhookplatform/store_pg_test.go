package webhookplatform

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"digital-contracting-service/internal/base/pgtest"
)

// DCS-NFR-PER-02: with a NATS queue group, the replica that dispatches an event
// is not always the replica that receives the subscriber callback. Each replica
// has its own PostgresDeliveries over the shared database, and both must see
// the same dispatch. Runs only against a real database (QA_DATABASE_URL).
func TestPostgresDeliveriesAreSharedAcrossReplicas(t *testing.T) {
	db := pgtest.Open(t)

	ctx := context.Background()
	dispatching, receiving := PostgresDeliveries{DB: db}, PostgresDeliveries{DB: db}
	owner := "did:web:" + uuid.NewString()
	pending := PendingCallback{EventID: uuid.NewString(), CorrelationID: uuid.NewString(), Event: "contract.created", DID: "did:x", SentAt: time.Now().UTC()}

	if tracked, err := dispatching.TrackPending(ctx, pending); err != nil || !tracked {
		t.Fatalf("TrackPending = %v, %v; want a new dispatch", tracked, err)
	}
	// NFR-SF-01: a redelivered event carries the same id and is not dispatched again.
	if tracked, err := receiving.TrackPending(ctx, pending); err != nil || tracked {
		t.Fatalf("second TrackPending = %v, %v; want false: the event is dispatched already", tracked, err)
	}
	if err := dispatching.AddDelivery(ctx, Delivery{EventID: pending.EventID, CallbackURL: "https://a.example", OwnerDID: owner, StatusCode: 200, DeliveredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("AddDelivery: %v", err)
	}

	resolved, ok, err := receiving.ResolvePending(ctx, pending.EventID)
	if err != nil || !ok || resolved.CorrelationID != pending.CorrelationID {
		t.Fatalf("ResolvePending on another replica = %+v, %v, %v; want the pending dispatch", resolved, ok, err)
	}
	if _, ok, err := dispatching.ResolvePending(ctx, pending.EventID); err != nil || ok {
		t.Fatalf("second ResolvePending = %v, %v; want false: only the first callback resolves a dispatch", ok, err)
	}
	if _, ok, err := receiving.ResolvePending(ctx, "unknown"); err != nil || ok {
		t.Fatalf("ResolvePending(unknown) = %v, %v; want false", ok, err)
	}

	got, err := receiving.ListDeliveries(ctx, owner)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListDeliveries on another replica = %+v, %v; want the one delivery", got, err)
	}
	if !got[0].Acknowledged || got[0].Event != "contract.created" || got[0].StatusCode != 200 || got[0].CorrelationID != pending.CorrelationID {
		t.Fatalf("delivery = %+v, want an acknowledged contract.created delivery with HTTP 200", got[0])
	}
	if other, err := receiving.ListDeliveries(ctx, "did:web:other"); err != nil || len(other) != 0 {
		t.Fatalf("ListDeliveries(other org) = %+v, %v; want none", other, err)
	}
}
