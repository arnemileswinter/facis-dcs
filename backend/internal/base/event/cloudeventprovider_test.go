package event

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	cloudevent "github.com/cloudevents/sdk-go/v2/event"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

// DCS-NFR-PER-02: the replicas of one instance subscribe with the same queue
// group, so a webhook, a PDF regeneration or an auto-deploy runs once per
// event and not once per replica. A different consumer (another queue group)
// still sees every event.
func TestQueueGroupDeliversEachEventOncePerConsumer(t *testing.T) {
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go ns.Start()
	t.Cleanup(ns.Shutdown)
	require.True(t, ns.ReadyForConnections(5*time.Second))
	url := ns.ClientURL()

	var webhook, deploy atomic.Int32
	subscribe := func(group string, counter *atomic.Int32) {
		c, err := NewNatsSubClient("dcs", url, group)
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
		go func() { _ = c.Subscribe(func(cloudevent.Event) { counter.Add(1) }) }()
	}
	// Three replicas of the webhook consumer, one of the deploy consumer.
	subscribe("dcs.webhook", &webhook)
	subscribe("dcs.webhook", &webhook)
	subscribe("dcs.webhook", &webhook)
	subscribe("dcs.deploy", &deploy)

	pub, err := NewNatsPubClient("dcs", url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	// The subscriptions open asynchronously; wait until all four are registered.
	require.Eventually(t, func() bool { return ns.NumSubscriptions() >= 4 }, 5*time.Second, 10*time.Millisecond)

	const events = 10
	for range events {
		require.NoError(t, pub.Publish("test", "CREATE_CONTRACT", json.RawMessage(`{"did":"x"}`)))
	}

	require.Eventually(t, func() bool { return deploy.Load() == events }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return webhook.Load() >= events }, 5*time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	require.EqualValues(t, events, webhook.Load(), "each event must reach one webhook replica, not every replica")
	require.EqualValues(t, events, deploy.Load())
}
