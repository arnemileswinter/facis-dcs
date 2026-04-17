package event

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
)

type OutboxProcessor struct {
	DB       *sqlx.DB
	Ctx      context.Context
	NatsConn *nats.Conn
}

func (j OutboxProcessor) Start() {
	go startProcessingJob(j.DB, j.Ctx, j.NatsConn, conf.OutboxProcessorTimeOut())
}

func startProcessingJob(db *sqlx.DB, ctx context.Context, natsConn *nats.Conn, interval time.Duration) {
	if natsConn == nil {
		return
	}

	schedulerLogic := func() error {
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not start transaction: %w", err)
		}
		defer tx.Rollback()

		rows, err := tx.QueryxContext(ctx, `
			SELECT id, component, event_type, event_data, did, created_at
			FROM outbox_events
			WHERE processed = FALSE
			ORDER BY created_at ASC
			LIMIT 100
			FOR UPDATE SKIP LOCKED
		`)
		if err != nil {
			return fmt.Errorf("could not query outbox events: %w", err)
		}

		type OutboxEvent struct {
			ID        int64     `db:"id"`
			Component string    `db:"component"`
			EventType string    `db:"event_type"`
			EventData []byte    `db:"event_data"`
			DID       *string   `db:"did"`
			CreatedAt time.Time `db:"created_at"`
		}

		var events []OutboxEvent
		for rows.Next() {
			var event OutboxEvent
			if err := rows.StructScan(&event); err != nil {
				rows.Close()
				return fmt.Errorf("could not scan event: %w", err)
			}
			events = append(events, event)
		}
		rows.Close()

		log.Println("process ", len(events), " events")

		for _, event := range events {
			subject := fmt.Sprintf("%s.%s", event.Component, event.EventType)
			if err := natsConn.Publish(subject, event.EventData); err != nil {
				return fmt.Errorf("could not publish event %d: %w", event.ID, err)
			}

			_, err := tx.ExecContext(ctx, `
				UPDATE outbox_events 
				SET processed = TRUE, processed_at = CURRENT_TIMESTAMP
				WHERE id = $1
			`, event.ID)
			if err != nil {
				return fmt.Errorf("could not mark event %d as processed: %w", event.ID, err)
			}
		}

		return tx.Commit()
	}

	ticker := time.NewTicker(interval)
	for range ticker.C {
		err := schedulerLogic()
		if err != nil {
			log.Printf("could not process outbox entries: %v", err)
		}
	}
}
