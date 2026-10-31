package webhookplatform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// KnownEvents is the list of subscribable DCS contract lifecycle events.
var KnownEvents = []EventInfo{
	{Name: "contract.created", Description: "A new contract was created"},
	{Name: "contract.submitted", Description: "A contract was submitted for review"},
	{Name: "contract.approved", Description: "A contract was approved"},
	{Name: "contract.rejected", Description: "A contract was rejected"},
	{Name: "contract.negotiated", Description: "A contract entered negotiation"},
	{Name: "contract.terminated", Description: "A contract was terminated"},
	// DCS-FR-CSA-20 / DCS-FR-CWE-10: API-delivered expiry alert; emitted by
	// the contractworkflowengine expiry cron when it persists EXPIRED.
	{Name: "contract.expired", Description: "A contract passed its expiration date"},
	// DCS-FR-CWE-22 / DCS-FR-CSA-20: renewal-deadline alert; emitted by the
	// expiry cron once per expiry date when the notice period starts.
	{Name: "contract.renewal_due", Description: "The notice period before a contract's expiration date started"},
	// DCS-FR-CSA-23: the other state changes a contract owner follows.
	{Name: "contract.offered", Description: "A contract was offered to the counterparty"},
	{Name: "contract.withdrawn", Description: "A contract offer was withdrawn"},
	{Name: "contract.signature_applied", Description: "A signature was applied to a contract"},
	{Name: "contract.signature_revoked", Description: "A contract signature was revoked"},
	{Name: "contract.activated", Description: "The target system acknowledged the deployment and the contract is active"},
	{Name: "contract.renewed", Description: "A contract was renewed"},
	{Name: "template.created", Description: "A new contract template was created"},
	{Name: "template.approved", Description: "A contract template was approved"},
	// DCS-FR-TR-22: Template Users subscribe to these to learn that a
	// template they have used was updated or re-registered as a new
	// version; the payload carries the template DID to filter on.
	{Name: "template.updated", Description: "A contract template was updated"},
	{Name: "template.registered", Description: "A contract template version was registered (published)"},
	{Name: "template.deprecated", Description: "A contract template was deprecated (archived)"},
	// DCS-FR-PACM-02: the "alert" half of "flagged and reported in real-time
	// via dashboards and alerts". Emitted by the continuous-monitoring sweep
	// the first time it detects a given violation, not on every run — see
	// querymonitor.go's reconcile.
	{Name: "compliance.risk_detected", Description: "Continuous monitoring detected a compliance risk"},
}

// DCSEventMap maps internal NATS event types to webhook event names.
var DCSEventMap = map[string]string{
	"CREATE_CONTRACT":            "contract.created",
	"SUBMIT_CONTRACT":            "contract.submitted",
	"APPROVE_CONTRACT":           "contract.approved",
	"REJECT_CONTRACT":            "contract.rejected",
	"NEGOTIATE_CONTRACT":         "contract.negotiated",
	"TERMINATE_CONTRACT":         "contract.terminated",
	"CONTRACT_EXPIRED":           "contract.expired",
	"CONTRACT_RENEWAL_DUE":       "contract.renewal_due",
	"OFFER_CONTRACT":             "contract.offered",
	"WITHDRAW_CONTRACT":          "contract.withdrawn",
	"APPLIED_SIGNATURE":          "contract.signature_applied",
	"REVOKE_SIGNATURE":           "contract.signature_revoked",
	"ACTIVATE_CONTRACT":          "contract.activated",
	"RENEW_CONTRACT":             "contract.renewed",
	"CREATE_CONTRACT_TEMPLATE":   "template.created",
	"APPROVE_CONTRACT_TEMPLATE":  "template.approved",
	"UPDATE_CONTRACT_TEMPLATE":   "template.updated",
	"REGISTER_CONTRACT_TEMPLATE": "template.registered",
	"ARCHIVE_CONTRACT_TEMPLATE":  "template.deprecated",
	"PAC_COMPLIANCE_RISK":        "compliance.risk_detected",
}

// EventInfo describes a subscribable event.
type EventInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Subscription represents a registered webhook.
//
// Secret is the plaintext signing secret. It exists only in memory: Postgres
// holds it encrypted, and only the POST /webhooks response carries it.
type Subscription struct {
	ID          string    `db:"id"`
	Event       string    `db:"event"`
	CallbackURL string    `db:"callback_url"`
	Secret      string    `db:"-"`
	OwnerDID    string    `db:"owner_did"`
	CreatedBy   string    `db:"created_by"`
	CreatedAt   time.Time `db:"created_at"`
}

// Subscriptions stores webhook subscriptions. Every caller-facing operation is
// scoped to the owning organization; ByEvent serves delivery only.
type Subscriptions interface {
	Add(ctx context.Context, sub Subscription) (Subscription, error)
	ListByOwner(ctx context.Context, ownerDID string) ([]Subscription, error)
	// DeleteOwned returns false when no subscription with that id belongs to ownerDID.
	DeleteOwned(ctx context.Context, id, ownerDID string) (bool, error)
	ByEvent(ctx context.Context, event string) ([]Subscription, error)
}

// PostgresSubscriptions stores subscriptions in webhook_subscriptions, with
// the secret encrypted by Secrets.
type PostgresSubscriptions struct {
	DB      *sqlx.DB
	Secrets SecretBox
}

const subscriptionColumns = `id, event, callback_url, owner_did, created_by, created_at`

func (s PostgresSubscriptions) Add(ctx context.Context, sub Subscription) (Subscription, error) {
	// The id is drawn here because the ciphertext is bound to it.
	id := uuid.New().String()
	ciphertext, wrappedCEK, err := s.Secrets.Seal(id, sub.Secret)
	if err != nil {
		return Subscription{}, err
	}
	var created Subscription
	err = s.DB.GetContext(ctx, &created, `
        INSERT INTO webhook_subscriptions (id, event, callback_url, secret_ciphertext, secret_wrapped_cek, owner_did, created_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING `+subscriptionColumns,
		id, sub.Event, sub.CallbackURL, ciphertext, wrappedCEK, sub.OwnerDID, sub.CreatedBy)
	if err != nil {
		return Subscription{}, fmt.Errorf("create webhook subscription: %w", err)
	}
	created.Secret = sub.Secret
	return created, nil
}

func (s PostgresSubscriptions) ListByOwner(ctx context.Context, ownerDID string) ([]Subscription, error) {
	subs := []Subscription{}
	if err := s.DB.SelectContext(ctx, &subs, `
        SELECT `+subscriptionColumns+` FROM webhook_subscriptions
        WHERE owner_did = $1 ORDER BY created_at, id`, ownerDID); err != nil {
		return nil, fmt.Errorf("list webhook subscriptions: %w", err)
	}
	return subs, nil
}

func (s PostgresSubscriptions) DeleteOwned(ctx context.Context, id, ownerDID string) (bool, error) {
	if _, err := uuid.Parse(id); err != nil {
		return false, nil
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM webhook_subscriptions WHERE id = $1 AND owner_did = $2`, id, ownerDID)
	if err != nil {
		return false, fmt.Errorf("delete webhook subscription %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete webhook subscription %s: %w", id, err)
	}
	return n > 0, nil
}

// ByEvent returns the subscriptions with their decrypted secrets.
func (s PostgresSubscriptions) ByEvent(ctx context.Context, event string) ([]Subscription, error) {
	var rows []struct {
		Subscription
		Ciphertext []byte `db:"secret_ciphertext"`
		WrappedCEK []byte `db:"secret_wrapped_cek"`
	}
	if err := s.DB.SelectContext(ctx, &rows, `
        SELECT `+subscriptionColumns+`, secret_ciphertext, secret_wrapped_cek
        FROM webhook_subscriptions WHERE event = $1`, event); err != nil {
		return nil, fmt.Errorf("list webhook subscriptions for %s: %w", event, err)
	}
	subs := make([]Subscription, 0, len(rows))
	for _, row := range rows {
		// One unreadable secret must not stop delivery to the other subscribers.
		secret, err := s.Secrets.Open(row.ID, row.Ciphertext, row.WrappedCEK)
		if err != nil {
			log.Printf("webhookplatform: skip subscription %s: %v", row.ID, err)
			continue
		}
		row.Secret = secret
		subs = append(subs, row.Subscription)
	}
	return subs, nil
}

// PendingCallback tracks an in-flight webhook dispatch awaiting a callback.
type PendingCallback struct {
	EventID       string
	CorrelationID string
	Event         string
	DID           string
	SentAt        time.Time
}

// Delivery is the recorded outcome of one webhook notification attempt —
// the observable behind GET /deliveries (monitoring and BDD assertions).
type Delivery struct {
	EventID       string    `json:"event_id" db:"event_id"`
	CorrelationID string    `json:"correlation_id" db:"correlation_id"`
	Event         string    `json:"event" db:"event"`
	DID           string    `json:"did" db:"did"`
	CallbackURL   string    `json:"callback_url" db:"callback_url"`
	OwnerDID      string    `json:"-" db:"owner_did"`
	StatusCode    int       `json:"status_code,omitempty" db:"status_code"`
	Error         string    `json:"error,omitempty" db:"error"`
	DeliveredAt   time.Time `json:"delivered_at" db:"delivered_at"`
	Acknowledged  bool      `json:"acknowledged" db:"acknowledged"`
}

// maxDeliveries bounds how many entries ListDeliveries returns (newest kept).
const maxDeliveries = 512

// Deliveries stores pending callbacks and the delivery log.
type Deliveries interface {
	// TrackPending records an in-flight dispatch so its callback can be correlated.
	TrackPending(ctx context.Context, p PendingCallback) error
	// ResolvePending marks the dispatch for eventID as acknowledged and returns
	// it. Only the first callback for an event_id resolves it.
	ResolvePending(ctx context.Context, eventID string) (PendingCallback, bool, error)
	// AddDelivery records a notification outcome.
	AddDelivery(ctx context.Context, d Delivery) error
	// ListDeliveries returns the newest deliveries to ownerDID's subscriptions,
	// newest last.
	ListDeliveries(ctx context.Context, ownerDID string) ([]Delivery, error)
}

// PostgresDeliveries stores pending callbacks and the delivery log in
// webhook_dispatches and webhook_deliveries, so every replica of the instance
// sees the same state (DCS-NFR-PER-02).
type PostgresDeliveries struct {
	DB *sqlx.DB
}

func (s PostgresDeliveries) TrackPending(ctx context.Context, p PendingCallback) error {
	if _, err := s.DB.ExecContext(ctx, `
        INSERT INTO webhook_dispatches (event_id, correlation_id, event, did, sent_at)
        VALUES ($1, $2, $3, $4, $5)`,
		p.EventID, p.CorrelationID, p.Event, p.DID, p.SentAt); err != nil {
		return fmt.Errorf("record webhook dispatch %s: %w", p.EventID, err)
	}
	return nil
}

func (s PostgresDeliveries) ResolvePending(ctx context.Context, eventID string) (PendingCallback, bool, error) {
	var p PendingCallback
	err := s.DB.QueryRowxContext(ctx, `
        UPDATE webhook_dispatches SET acknowledged_at = now()
        WHERE event_id = $1 AND acknowledged_at IS NULL
        RETURNING event_id, correlation_id, event, did, sent_at`, eventID).
		Scan(&p.EventID, &p.CorrelationID, &p.Event, &p.DID, &p.SentAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingCallback{}, false, nil
	}
	if err != nil {
		return PendingCallback{}, false, fmt.Errorf("resolve webhook dispatch %s: %w", eventID, err)
	}
	return p, true, nil
}

func (s PostgresDeliveries) AddDelivery(ctx context.Context, d Delivery) error {
	if _, err := s.DB.ExecContext(ctx, `
        INSERT INTO webhook_deliveries (event_id, callback_url, owner_did, status_code, error, delivered_at)
        VALUES ($1, $2, $3, $4, $5, $6)`,
		d.EventID, d.CallbackURL, d.OwnerDID, d.StatusCode, d.Error, d.DeliveredAt); err != nil {
		return fmt.Errorf("record webhook delivery %s: %w", d.EventID, err)
	}
	return nil
}

func (s PostgresDeliveries) ListDeliveries(ctx context.Context, ownerDID string) ([]Delivery, error) {
	out := []Delivery{}
	if err := s.DB.SelectContext(ctx, &out, `
        SELECT event_id, correlation_id, event, did, callback_url, owner_did, status_code, error, delivered_at, acknowledged
        FROM (
            SELECT d.id, d.event_id, p.correlation_id, p.event, p.did, d.callback_url, d.owner_did,
                   d.status_code, d.error, d.delivered_at, p.acknowledged_at IS NOT NULL AS acknowledged
            FROM webhook_deliveries d JOIN webhook_dispatches p ON p.event_id = d.event_id
            WHERE d.owner_did = $1
            ORDER BY d.id DESC LIMIT $2
        ) newest ORDER BY id`, ownerDID, maxDeliveries); err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	return out, nil
}
