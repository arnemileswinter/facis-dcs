package datatype

import "time"

type OutboxEvent struct {
	ID        int64     `db:"id"         json:"id"`
	Component string    `db:"component"  json:"component"`
	EventType string    `db:"event_type" json:"event_type"`
	EventData []byte    `db:"event_data" json:"event_data"`
	DID       *string   `db:"did"        json:"did"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
