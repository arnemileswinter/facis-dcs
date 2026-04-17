package event

import (
	"log"

	"github.com/nats-io/nats.go"
)

type NatsDebugConsumer struct {
	NatsConn *nats.Conn
}

func (j NatsDebugConsumer) Start() {
	j.NatsConn.Subscribe(">", func(msg *nats.Msg) {
		log.Printf("Subject: %s | Data: %s", msg.Subject, string(msg.Data))
	})
}
