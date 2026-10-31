package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"digital-contracting-service/internal/base/db"
)

// OutboxConsumer hands each outbox event of Types to Handle once, read from
// the outbox table and not from NATS: core NATS drops an event that no member
// of a queue group receives, for example while the replicas restart
// (DCS-NFR-SF-01). One replica reads at a time, in outbox order. An event is
// marked handled after Handle returns, so a replica that stops in between
// hands it over again.
//
// ponytail: reads the events of the last day only; an event whose transaction
// committed later than that is not handed over.
type OutboxConsumer struct {
	DB     *sqlx.DB
	Name   string
	Types  []string
	Handle func(ctx context.Context, eventType string, data json.RawMessage)
}

const outboxConsumerBatch = 50

// Start polls the outbox every interval until ctx ends.
func (c *OutboxConsumer) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Poll(ctx); err != nil {
					log.Printf("outbox consumer %s: %v", c.Name, err)
				}
			}
		}
	}()
}

// Poll hands the events not handled yet to Handle.
func (c *OutboxConsumer) Poll(ctx context.Context) error {
	_, err := db.RunExclusive(ctx, c.DB, "outbox-consumer:"+c.Name, func() error {
		var events []struct {
			ID        int64           `db:"id"`
			EventType string          `db:"event_type"`
			EventData json.RawMessage `db:"event_data"`
		}
		if err := c.DB.SelectContext(ctx, &events, `
			SELECT o.id, o.event_type, o.event_data FROM outbox_events o
			WHERE o.event_type = ANY($2) AND o.created_at > now() - interval '1 day'
			  AND NOT EXISTS (SELECT 1 FROM outbox_consumer_progress p WHERE p.consumer = $1 AND p.outbox_event_id = o.id)
			ORDER BY o.id LIMIT $3`, c.Name, pq.Array(c.Types), outboxConsumerBatch); err != nil {
			return fmt.Errorf("read the outbox: %w", err)
		}
		for _, evt := range events {
			c.Handle(ctx, evt.EventType, evt.EventData)
			if _, err := c.DB.ExecContext(ctx, `
				INSERT INTO outbox_consumer_progress (consumer, outbox_event_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, c.Name, evt.ID); err != nil {
				return fmt.Errorf("mark outbox event %d handled: %w", evt.ID, err)
			}
		}
		return nil
	})
	return err
}
