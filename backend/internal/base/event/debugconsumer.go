package event

import (
	"log"

	"github.com/cloudevents/sdk-go/v2/event"
)

type EventDebugConsumer struct {
	SubClient *CloudEventSubClient
}

func (j EventDebugConsumer) Start() {
	go func() {
		j.SubClient.Subscribe(func(evt event.Event) {
			data := evt.Data()
			evt.ID()
			log.Printf("Subject: %s | Data: %s", evt.ID(), string(data))
		})
	}()
}

func (j EventDebugConsumer) Stop() {
	j.SubClient.cancel()
}
