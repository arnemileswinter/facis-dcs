package event

import (
	"context"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Event interface {
	// EventType returns the name of the event (used as NATS subject).
	EventType() string

	// GetDID returns the entity DID for event reference and correlation.
	GetDID() string
}

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
		return errors.New("did cannot be empty")
	}

	eventJSON, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

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
