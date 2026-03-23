package eventbus

type EventBus interface {
	Publish(subj string, data any) error
	SubscribeAsync(subj string, handler func(data []byte)) error
}
