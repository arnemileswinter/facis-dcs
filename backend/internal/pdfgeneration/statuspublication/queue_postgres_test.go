package statuspublication

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"digital-contracting-service/internal/pdfgeneration/provenance"
	"digital-contracting-service/migrations"
)

type failingPublisher struct{ provenance.StatusListPublisher }

func (failingPublisher) PublishStatus(context.Context, string, string, string, time.Time) (provenance.CredentialStatusRef, error) {
	return provenance.CredentialStatusRef{}, errors.New("status list store unavailable")
}

// A terminal contract state enqueued with the transition reaches the status
// list this deployment serves (DCS-IR-SI-09): a failed delivery stays pending
// with its error, and the next delivery sets the contract's revocation bit.
func TestQueuedTerminationSetsTheServedRevocationBit(t *testing.T) {
	db := queueTestDB(t)
	ctx := context.Background()
	const contract = "did:web:dcs.example:contract:terminated"

	for range 2 {
		tx := db.MustBeginTx(ctx, nil)
		if err := EnqueueTx(ctx, tx, contract, "TERMINATED", "end", time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	failing := &Worker{DB: db, Publisher: failingPublisher{}}
	if processed, err := failing.ProcessOnce(ctx); err != nil || !processed {
		t.Fatalf("failed delivery: processed=%v err=%v", processed, err)
	}
	var row struct {
		Pending   bool    `db:"pending"`
		LastError *string `db:"last_error"`
	}
	if err := db.Get(&row, `SELECT published_at IS NULL AS pending, last_error FROM status_publication_queue WHERE contract_did = $1`, contract); err != nil {
		t.Fatal(err)
	}
	if !row.Pending || row.LastError == nil {
		t.Fatalf("a failed delivery must stay pending with its error, got %+v", row)
	}
	if _, err := db.Exec(`UPDATE status_publication_queue SET next_attempt_at = CURRENT_TIMESTAMP - INTERVAL '1 second'`); err != nil {
		t.Fatal(err)
	}

	revocations := &provenance.PostgresStatusListRevocations{DB: db}
	allocator := provenance.NewPostgresStatusListAllocator(db, provenance.DefaultListID)
	publisher := provenance.NewDCSStatusListPublisher(
		func(listID int) string { return provenance.StatusListURI("https://dcs.example", listID) }, allocator, revocations)
	worker := &Worker{DB: db, Publisher: publisher}
	if processed, err := worker.ProcessOnce(ctx); err != nil || !processed {
		t.Fatalf("delivery: processed=%v err=%v", processed, err)
	}
	if processed, _ := worker.ProcessOnce(ctx); processed {
		t.Fatal("a delivered entry must not be delivered again")
	}

	entry, err := allocator.Allocate(ctx, contract)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := revocations.RevokedIndices(ctx, entry.ListID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(revoked, entry.Index) {
		t.Fatalf("revoked indices %v of list %d do not contain the terminated contract's index %d", revoked, entry.ListID, entry.Index)
	}
}

// queueTestDB migrates a temporary schema of the database QA_DATABASE_URL names.
func queueTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("QA_DATABASE_URL")
	if dsn == "" {
		t.Skip("set QA_DATABASE_URL to a disposable PostgreSQL database")
	}
	admin, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("status_publication_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE") })
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := sqlx.Connect("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}
