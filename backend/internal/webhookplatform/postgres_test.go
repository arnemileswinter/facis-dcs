package webhookplatform

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"digital-contracting-service/migrations"
)

// Runs only against a real database: QA_DATABASE_URL must point to an empty
// Postgres that the migrations may write to.
func TestPostgresSubscriptionsScopeToOwner(t *testing.T) {
	url := os.Getenv("QA_DATABASE_URL")
	if url == "" {
		t.Skip("QA_DATABASE_URL is not set")
	}
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM webhook_subscriptions`); err != nil {
		t.Fatalf("clean: %v", err)
	}

	ctx := context.Background()
	s := PostgresSubscriptions{DB: db}
	a, err := s.Add(ctx, Subscription{Event: "template.updated", CallbackURL: "https://a.example", Secret: "sa", OwnerDID: orgA, CreatedBy: "user-a"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := s.Add(ctx, Subscription{Event: "contract.created", CallbackURL: "https://b.example", OwnerDID: orgB, CreatedBy: "user-b"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	owned, err := s.ListByOwner(ctx, orgA)
	if err != nil || len(owned) != 1 || owned[0].ID != a.ID {
		t.Fatalf("ListByOwner(orgA) = %+v, %v; want only %s", owned, err, a.ID)
	}
	byEvent, err := s.ByEvent(ctx, "template.updated")
	if err != nil || len(byEvent) != 1 || byEvent[0].Secret != "sa" {
		t.Fatalf("ByEvent = %+v, %v; want org A's subscription with its secret", byEvent, err)
	}

	if ok, err := s.DeleteOwned(ctx, a.ID, orgB); err != nil || ok {
		t.Fatalf("DeleteOwned by orgB = %v, %v; want false", ok, err)
	}
	if ok, err := s.DeleteOwned(ctx, "not-a-uuid", orgA); err != nil || ok {
		t.Fatalf("DeleteOwned(not-a-uuid) = %v, %v; want false, nil", ok, err)
	}
	if ok, err := s.DeleteOwned(ctx, a.ID, orgA); err != nil || !ok {
		t.Fatalf("DeleteOwned by owner = %v, %v; want true", ok, err)
	}
}
