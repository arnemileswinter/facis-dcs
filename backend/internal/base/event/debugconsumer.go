package event

import (
	"encoding/base64"
	"log"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
)

type EventDebugConsumer struct {
	SubClient *CloudEventSubClient
}

func (j EventDebugConsumer) Start() {
	go func() {
		j.SubClient.Subscribe(func(evt event.Event) {

			raw := string(evt.Data())

			raw = strings.Trim(raw, `"`)

			payload, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				log.Printf("Receive Event Message - ID: %s,| Source: %s | Type: %s | could not decode payload", evt.ID(), evt.Source(), evt.Type())
				return
			}

			log.Printf("Receive Event Message - ID: %s,| Source: %s | Type: %s | Payload: %s", evt.ID(), evt.Source(), evt.Type(), payload)
		})
	}()
}

func (j EventDebugConsumer) Stop() {
	j.SubClient.cancel()
}
