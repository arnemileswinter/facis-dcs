package webhookplatform

import (
	"context"
	"encoding/json"
	"log"
	"maps"
	"slices"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/event"
)

// OutboxConsumer dispatches every DCS event a webhook may carry, read from the
// outbox table (event.OutboxConsumer), so an event published while no replica
// listens on NATS is still delivered (DCS-NFR-SF-01, DCS-IR-SI-02).
func OutboxConsumer(database *sqlx.DB, d *Dispatcher) *event.OutboxConsumer {
	return &event.OutboxConsumer{
		DB:    database,
		Name:  "webhooks",
		Types: slices.Sorted(maps.Keys(DCSEventMap)),
		Handle: func(ctx context.Context, eventType string, data json.RawMessage) {
			handleDCSEvent(ctx, eventType, data, d)
		},
	}
}

func handleDCSEvent(ctx context.Context, eventType string, data json.RawMessage, d *Dispatcher) {
	if _, known := DCSEventMap[eventType]; !known {
		return
	}
	var envelope struct {
		DID string `json:"did"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		log.Printf("webhookplatform: unmarshal DID from %s payload: %v", eventType, err)
		return
	}
	d.DispatchFromDCS(ctx, eventType, envelope.DID, data)
}
