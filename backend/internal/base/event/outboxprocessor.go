package event

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/processauditandcompliance/db"
	"digital-contracting-service/internal/processauditandcompliance/ipfs"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

const GlobalAuditTrail string = "GLOBAL_AUDIT_TRAIL"

type OutboxProcessor struct {
	DB         *sqlx.DB
	PubClient  *CloudEventPubClient
	IPFSClient *ipfs.APIClient
	Repo       db.AuditAndComplianceRepository
}

type OutboxEvent struct {
	ID        int64     `db:"id"         json:"id"`
	Component string    `db:"component"  json:"component"`
	EventType string    `db:"event_type" json:"event_type"`
	EventData []byte    `db:"event_data" json:"event_data"`
	DID       *string   `db:"did"        json:"did"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type AuditLogEntry struct {
	ID               int64     `json:"id"`
	Component        string    `json:"component"`
	EventType        string    `json:"event_type"`
	EventData        []byte    `json:"event_data"`
	DID              *string   `json:"did"`
	CreatedAt        time.Time `json:"created_at"`
	ResLogPredCID    *string   `json:"res_log_pred_cid"`
	GlobalLogPredCID *string   `json:"global_log_pred_cid"`
}

func (j OutboxProcessor) Start(ctx context.Context) error {
	go j.startProcessingJob(ctx, conf.OutboxProcessorTimeOut())
	return nil
}

func (j OutboxProcessor) startProcessingJob(ctx context.Context, interval time.Duration) {
	if j.PubClient == nil {
		return
	}

	schedulerLogic := func() error {
		tx, err := j.DB.BeginTxx(ctx, nil)
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

		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("could not commit transaction: %w", err)
		}

		if len(events) > 0 {
			log.Println("process ", len(events), " events")
		}

		for _, event := range events {
			if err := j.processEvent(ctx, event); err != nil {
				log.Printf("could not process event %d: %v", event.ID, err)
				return err
			}
		}

		return nil
	}

	ticker := time.NewTicker(interval)
	for range ticker.C {
		if err := schedulerLogic(); err != nil {
			log.Printf("could not process outbox entries: %v", err)
		}
	}
}

func (j OutboxProcessor) processEvent(ctx context.Context, event OutboxEvent) error {
	tx, err := j.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	if err := j.PubClient.Publish(event.Component, event.EventType, event.EventData); err != nil {
		return fmt.Errorf("could not publish event %d: %w", event.ID, err)
	}

	globalLogPredCID, err := j.Repo.ReadLogCID(ctx, tx, componenttype.AuditAndCompliance.String(), GlobalAuditTrail)
	if err != nil {
		return fmt.Errorf("could not read log CID: %w", err)
	}

	var resLogPredCID *string
	switch event.Component {
	case componenttype.ContractTemplateRepo.String():
		if event.DID != nil && len(*event.DID) > 1 {
			resLogPredCID, err = j.Repo.ReadLogCID(ctx, tx, event.Component, *event.DID)
			if err != nil {
				return fmt.Errorf("could not read log CID: %w", err)
			}
		}
	case componenttype.ContractWorkflowEngine.String():
		if event.DID != nil && len(*event.DID) > 1 {
			resLogPredCID, err = j.Repo.ReadLogCID(ctx, tx, event.Component, *event.DID)
			if err != nil {
				return fmt.Errorf("could not read log CID: %w", err)
			}
		}
	}

	auditLogEntry := AuditLogEntry{
		ID:               event.ID,
		Component:        event.Component,
		EventType:        event.EventType,
		EventData:        event.EventData,
		DID:              event.DID,
		CreatedAt:        event.CreatedAt,
		ResLogPredCID:    resLogPredCID,
		GlobalLogPredCID: globalLogPredCID,
	}

	result, err := j.IPFSClient.CreateFile(ctx, auditLogEntry)
	if err != nil {
		return fmt.Errorf("could not create IPFS file for event %d: %w", event.ID, err)
	}
	globalLogPredCID = &result.Identifier.Value

	switch event.Component {
	case componenttype.ContractTemplateRepo.String():
		if event.DID != nil && len(*event.DID) > 1 {
			if err = j.Repo.UpdateLogCID(ctx, tx, event.Component, *event.DID, &result.Identifier.Value); err != nil {
				return fmt.Errorf("could not update log CID: %w", err)
			}
		}
	case componenttype.ContractWorkflowEngine.String():
		if event.DID != nil && len(*event.DID) > 1 {
			if err = j.Repo.UpdateLogCID(ctx, tx, event.Component, *event.DID, &result.Identifier.Value); err != nil {
				return fmt.Errorf("could not update log CID: %w", err)
			}
		}
	}

	if err = j.Repo.UpdateLogCID(ctx, tx, componenttype.AuditAndCompliance.String(), GlobalAuditTrail, &result.Identifier.Value); err != nil {
		return fmt.Errorf("could not update log CID: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
        UPDATE outbox_events 
        SET processed = TRUE, processed_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `, event.ID); err != nil {
		return fmt.Errorf("could not mark event %d as processed: %w", event.ID, err)
	}

	return tx.Commit()
}
