package event

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/pgtest"
)

// NFR-26 (DCS-NFR-SF-01): an outbox consumer reads its events from the outbox
// table, whether or not NATS delivered them to a replica, hands each one over
// once in outbox order, and leaves the other event types alone.
func TestOutboxConsumerHandsEachEventOverOnce(t *testing.T) {
	database := pgtest.Open(t)
	ctx := context.Background()
	insert := func(eventType, did string) {
		_, err := database.Exec(`INSERT INTO outbox_events (component, event_type, event_data, did, published) VALUES ('SYSTEM', $1, $2, $3, TRUE)`,
			eventType, `{"did":"`+did+`"}`, did)
		require.NoError(t, err)
	}
	insert("APPLIED_SIGNATURE", "did:a")
	insert("CREATE_CONTRACT", "did:b")
	insert("APPLIED_SIGNATURE", "did:c")

	var handled []string
	consumer := &OutboxConsumer{DB: database, Name: "test", Types: []string{"APPLIED_SIGNATURE"},
		Handle: func(_ context.Context, eventType string, data json.RawMessage) {
			var evt struct {
				DID string `json:"did"`
			}
			require.NoError(t, json.Unmarshal(data, &evt))
			handled = append(handled, eventType+" "+evt.DID)
		}}

	require.NoError(t, consumer.Poll(ctx))
	require.NoError(t, consumer.Poll(ctx))
	require.Equal(t, []string{"APPLIED_SIGNATURE did:a", "APPLIED_SIGNATURE did:c"}, handled)

	insert("APPLIED_SIGNATURE", "did:d")
	require.NoError(t, consumer.Poll(ctx))
	require.Equal(t, "APPLIED_SIGNATURE did:d", handled[len(handled)-1])
	require.Len(t, handled, 3)

	other := &OutboxConsumer{DB: database, Name: "other", Types: []string{"APPLIED_SIGNATURE"},
		Handle: func(context.Context, string, json.RawMessage) { handled = append(handled, "other") }}
	require.NoError(t, other.Poll(ctx))
	require.Len(t, handled, 6, "each consumer keeps its own progress")
}
