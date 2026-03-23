package event

import (
	"context"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Event interface must be implemented by all event types.
// This interface ensures consistency across event handling.
type Event interface {
	// EventType returns the name of the event (used as NATS subject).
	EventType() string

	// GetDID returns the entity DID for event reference and correlation.
	GetDID() string
}

// Create persists an event to the outbox table.
// This function is called by command handlers to store events durably
// before they are published to NATS.
//
// IMPORTANT: Must be called within the same database transaction as the
// command that triggered the event. This ensures atomicity: either both
// the command and event succeed, or both are rolled back.
//
// Usage in a command handler:
//
//	evt := TemplateCreatedEvent{
//	    TemplateID: "123",
//	    CreatedBy:  "user@example.com",
//	    OccurredAt: time.Now(),
//	}
//	if err := event.Create(ctx, tx, evt); err != nil {
//	    return err
//	}
func Create(ctx context.Context, tx *sqlx.Tx, evt Event, component componenttype.ComponentType) error {
	if evt == nil {
		return errors.New("event cannot be nil")
	}

	eventType := evt.EventType()
	if eventType == "" {
		return errors.New("event type cannot be empty")
	}

	did := evt.GetDID()
	if did == "" {
		return errors.New("template did cannot be empty")
	}

	// Serialize event to JSON
	eventJSON, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Insert into outbox (MUST be in same transaction as DB update!)
	// The outbox table ensures events are never lost, even if NATS is down.
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events 
		 (component, event_type, event_data, did, processed)
		 VALUES ($1, $2, $3, $4, FALSE)`,
		component.String(),
		eventType,
		eventJSON,
		did,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event into outbox: %w", err)
	}

	return nil
}

// CreateNewEvents persists multiple events to the outbox in a single operation.
// This is useful when a single command handler needs to emit multiple events.
//
// Events are inserted sequentially within the same transaction.
//
// Usage:
//
//	evts := []event.Event{
//	    TemplateCreatedEvent{...},
//	    TemplateInitializedEvent{...},
//	    TemplateReadyEvent{...},
//	}
//	if err := event.CreateNewEvents(ctx, tx, evts...); err != nil {
//	    return err
//	}
func CreateNewEvents(ctx context.Context, tx *sqlx.Tx, component componenttype.ComponentType, events ...Event) error {
	if len(events) == 0 {
		return nil // Nothing to store
	}

	for _, evt := range events {
		if err := Create(ctx, tx, evt, component); err != nil {
			return err
		}
	}

	return nil
}
