package processauditandcompliance

import (
	"crypto/sha256"
	event2 "digital-contracting-service/internal/base/event"
	"encoding/base64"
	"log"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
)

type AuditLogEntry struct {
	LogEntry LogEntry `json:"log_entry"`
	Checksum [32]byte `json:"hash"`
}

type LogEntry struct {
	EventID      string   `json:"event_id"`
	Source       string   `json:"source"`
	EventType    string   `json:"event_type"`
	Payload      string   `json:"payload"`
	PredChecksum [32]byte `json:"pred_checksum"`
}

type PACSubscriber struct {
	SubClient *event2.CloudEventSubClient
}

func (j PACSubscriber) Start() {
	go func() {
		var previousChecksum [32]byte
		err := j.SubClient.Subscribe(func(evt event.Event) {

			raw := string(evt.Data())
			raw = strings.Trim(raw, `"`)

			payload, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				log.Println(err)
				return
			}

			logEntry := LogEntry{
				EventID:      evt.ID(),
				Source:       evt.Source(),
				EventType:    evt.Type(),
				Payload:      string(payload),
				PredChecksum: previousChecksum,
			}
			checksum := sha256.Sum256(payload)
			auditLogEntry := AuditLogEntry{
				LogEntry: logEntry,
				Checksum: checksum,
			}
			log.Println(auditLogEntry)

			previousChecksum = checksum
		})
		if err != nil {
			log.Fatalf("failed to subscribe to events: %s", err)
		}
	}()
}

func (j PACSubscriber) Stop() {
	j.SubClient.Cancel()
}
