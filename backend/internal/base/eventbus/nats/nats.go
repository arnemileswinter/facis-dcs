package nats

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type NatsEventBus struct {
	Conn *nats.Conn
}

func (eb *NatsEventBus) Publish(subj string, data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return eb.Conn.Publish(subj, bytes)
}

func (eb *NatsEventBus) SubscribeAsync(subj string, handler func(data []byte)) error {

	_, err := eb.Conn.Subscribe(subj, func(m *nats.Msg) {
		handler(m.Data)
	})
	if err != nil {
		return err
	}

	return nil
}
