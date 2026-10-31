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

	if err := dispatching.TrackPending(ctx, pending); err != nil {
		t.Fatalf("TrackPending: %v", err)
	}
	id, err := dispatching.QueueDelivery(ctx, Delivery{EventID: pending.EventID, SubscriptionID: uuid.NewString(), CallbackURL: "https://a.example", OwnerDID: owner, Payload: []byte(`{}`)}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("QueueDelivery: %v", err)
	}
	if err := dispatching.RecordAttempt(ctx, id, 200, "", time.Time{}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}

	if _, ok, err := receiving.ResolvePending(ctx, pending.EventID, "did:web:other"); err != nil || ok {
		t.Fatalf("ResolvePending by an organization that did not receive the event = %v, %v; want false", ok, err)
	}
	resolved, ok, err := receiving.ResolvePending(ctx, pending.EventID, owner)
	if err != nil || !ok || resolved.CorrelationID != pending.CorrelationID {
		t.Fatalf("ResolvePending on another replica = %+v, %v, %v; want the pending dispatch", resolved, ok, err)
	}
	if _, ok, err := dispatching.ResolvePending(ctx, pending.EventID, owner); err != nil || ok {
		t.Fatalf("second ResolvePending = %v, %v; want false: only the first callback resolves a dispatch", ok, err)
	}
	if _, ok, err := receiving.ResolvePending(ctx, "unknown", owner); err != nil || ok {
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

// UC-IF-27: a queued delivery is due at its retry time until an attempt ends
// it, and an ended delivery keeps no payload.
func TestPostgresDeliveriesAreDueUntilTheyEnd(t *testing.T) {
	db := pgtest.Open(t)
	ctx := context.Background()
	store := PostgresDeliveries{DB: db}
	pending := PendingCallback{EventID: uuid.NewString(), CorrelationID: uuid.NewString(), Event: "contract.created", DID: "did:x", SentAt: time.Now().UTC()}
	if err := store.TrackPending(ctx, pending); err != nil {
		t.Fatal(err)
	}
	id, err := store.QueueDelivery(ctx, Delivery{EventID: pending.EventID, SubscriptionID: uuid.NewString(), CallbackURL: "https://a.example", OwnerDID: "did:web:a", Payload: []byte(`{"a":1}`)}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if due, err := store.DueDeliveries(ctx, time.Now(), 10); err != nil || len(due) != 0 {
		t.Fatalf("a delivery is due before its retry time: %+v, %v", due, err)
	}
	due, err := store.DueDeliveries(ctx, time.Now().Add(2*time.Minute), 10)
	if err != nil || len(due) != 1 || string(due[0].Payload) != `{"a":1}` || due[0].CorrelationID != pending.CorrelationID {
		t.Fatalf("due deliveries = %+v, %v; want the queued delivery with its payload", due, err)
	}

	if err := store.RecordAttempt(ctx, id, 503, "", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if due, _ := store.DueDeliveries(ctx, time.Now().Add(2*time.Minute), 10); len(due) != 1 || due[0].Attempts != 1 {
		t.Fatalf("a refused delivery is not due again: %+v", due)
	}
	if err := store.RecordAttempt(ctx, id, 200, "", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if due, _ := store.DueDeliveries(ctx, time.Now().Add(24*time.Hour), 10); len(due) != 0 {
		t.Fatalf("an ended delivery is still due: %+v", due)
	}
	var payloads int
	if err := db.Get(&payloads, `SELECT count(*) FROM webhook_deliveries WHERE payload IS NOT NULL`); err != nil || payloads != 0 {
		t.Fatalf("an ended delivery keeps its payload: %d, %v", payloads, err)
	}
}

// NFR-26 (DCS-NFR-SF-01): a DCS event reaches the webhook subscribers from the
// outbox table, without NATS, and only once.
func TestWebhookDeliveriesAreReadFromTheOutbox(t *testing.T) {
	db := pgtest.Open(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO outbox_events (component, event_type, event_data, did) VALUES
		('CONTRACT_TEMPLATE_REPOSITORY', 'CREATE_CONTRACT_TEMPLATE', '{"did":"did:web:a.example:template:1"}', 'did:web:a.example:template:1')`); err != nil {
		t.Fatal(err)
	}
	store := newMemDeliveries()
	subs := &fakeSubscriptions{subs: []Subscription{{ID: "s", Event: "template.created", CallbackURL: "https://receiver.example/hook", Secret: "whsec_x", OwnerDID: orgA}}}
	consumer := OutboxConsumer(db, NewDispatcher(subs, store, admitAll, nil))

	for range 2 {
		if err := consumer.Poll(ctx); err != nil {
			t.Fatal(err)
		}
	}

	got, _ := store.ListDeliveries(ctx, orgA)
	if len(got) != 1 || got[0].Event != "template.created" || got[0].DID != "did:web:a.example:template:1" {
		t.Fatalf("deliveries = %+v, want one template.created delivery", got)
	}
}
