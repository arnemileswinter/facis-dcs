package webhookplatform

import (
	"context"
	"fmt"
	"sync"
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
// Secret is the bearer value sent to CallbackURL. The subscriber supplies it and
// already knows it, so no response carries it (see subscriptionView).
type Subscription struct {
	ID          string    `db:"id"`
	Event       string    `db:"event"`
	CallbackURL string    `db:"callback_url"`
	Secret      string    `db:"secret"`
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

// PostgresSubscriptions stores subscriptions in webhook_subscriptions.
type PostgresSubscriptions struct {
	DB *sqlx.DB
}

const subscriptionColumns = `id, event, callback_url, secret, owner_did, created_by, created_at`

func (s PostgresSubscriptions) Add(ctx context.Context, sub Subscription) (Subscription, error) {
	var created Subscription
	err := s.DB.GetContext(ctx, &created, `
        INSERT INTO webhook_subscriptions (event, callback_url, secret, owner_did, created_by)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING `+subscriptionColumns,
		sub.Event, sub.CallbackURL, sub.Secret, sub.OwnerDID, sub.CreatedBy)
	if err != nil {
		return Subscription{}, fmt.Errorf("create webhook subscription: %w", err)
	}
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

func (s PostgresSubscriptions) ByEvent(ctx context.Context, event string) ([]Subscription, error) {
	subs := []Subscription{}
	if err := s.DB.SelectContext(ctx, &subs, `
        SELECT `+subscriptionColumns+` FROM webhook_subscriptions WHERE event = $1`, event); err != nil {
		return nil, fmt.Errorf("list webhook subscriptions for %s: %w", event, err)
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
	EventID       string    `json:"event_id"`
	CorrelationID string    `json:"correlation_id"`
	Event         string    `json:"event"`
	DID           string    `json:"did"`
	CallbackURL   string    `json:"callback_url"`
	StatusCode    int       `json:"status_code,omitempty"`
	Error         string    `json:"error,omitempty"`
	DeliveredAt   time.Time `json:"delivered_at"`
	Acknowledged  bool      `json:"acknowledged"`
}

// maxDeliveries bounds the in-memory delivery log (newest kept).
const maxDeliveries = 512

// DeliveryLog is a thread-safe in-memory store for pending callbacks and the
// recent-delivery log.
type DeliveryLog struct {
	mu         sync.RWMutex
	pending    map[string]PendingCallback // event_id → pending
	deliveries []Delivery                 // newest last, capped at maxDeliveries
}

// NewDeliveryLog returns an empty log.
func NewDeliveryLog() *DeliveryLog {
	return &DeliveryLog{pending: make(map[string]PendingCallback)}
}

// TrackPending records an in-flight dispatch so its callback can be correlated.
func (s *DeliveryLog) TrackPending(p PendingCallback) {
	s.mu.Lock()
	s.pending[p.EventID] = p
	s.mu.Unlock()
}

// ResolvePending looks up and removes a pending callback by event_id, and
// flips the matching delivery-log entries to acknowledged.
func (s *DeliveryLog) ResolvePending(eventID string) (PendingCallback, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[eventID]
	if ok {
		delete(s.pending, eventID)
		for i := range s.deliveries {
			if s.deliveries[i].EventID == eventID {
				s.deliveries[i].Acknowledged = true
			}
		}
	}
	return p, ok
}

// AddDelivery appends a notification outcome to the delivery log.
func (s *DeliveryLog) AddDelivery(d Delivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, d)
	if len(s.deliveries) > maxDeliveries {
		s.deliveries = s.deliveries[len(s.deliveries)-maxDeliveries:]
	}
}

// ListDeliveries returns a snapshot of the delivery log, newest last.
func (s *DeliveryLog) ListDeliveries() []Delivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Delivery(nil), s.deliveries...)
}
