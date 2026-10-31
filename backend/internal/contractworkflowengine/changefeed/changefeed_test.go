package changefeed

import (
	"testing"

	cloudevent "github.com/cloudevents/sdk-go/v2/event"
	"github.com/stretchr/testify/require"
)

func busEvent(t *testing.T, eventType, data string) cloudevent.Event {
	t.Helper()
	evt := cloudevent.New()
	evt.SetID("1")
	evt.SetSource("dcs")
	evt.SetType(eventType)
	require.NoError(t, evt.SetData(cloudevent.ApplicationJSON, []byte(data)))
	return evt
}

// DCS-FR-CWE-24: a contract change reaches every subscriber; a read event
// does not, so a dashboard that refetches after a change starts no loop.
func TestFeedPassesChangesAndDropsReads(t *testing.T) {
	feed := New()
	first, stopFirst := feed.Subscribe()
	second, stopSecond := feed.Subscribe()
	defer stopSecond()

	feed.Handle(busEvent(t, "RETRIEVE_ALL_CONTRACTS", `{"did":"did:contract:1"}`))
	feed.Handle(busEvent(t, "SEARCH_CONTRACT", `{"did":"did:contract:1"}`))
	feed.Handle(busEvent(t, "SUBMIT_CONTRACT", `{"did":"did:contract:1"}`))

	for _, ch := range []<-chan Change{first, second} {
		require.Equal(t, Change{DID: "did:contract:1", EventType: "SUBMIT_CONTRACT"}, <-ch)
		require.Empty(t, ch)
	}

	stopFirst()
	feed.Handle(busEvent(t, "TERMINATE_CONTRACT", `{"did":"did:contract:1"}`))
	require.Empty(t, first)
	require.Equal(t, "TERMINATE_CONTRACT", (<-second).EventType)
}

// DCS-FR-CWE-24: the deployment acknowledgement that activates a contract
// reaches the dashboards.
func TestFeedPassesTheActivation(t *testing.T) {
	feed := New()
	changes, stop := feed.Subscribe()
	defer stop()

	feed.Handle(busEvent(t, "ACTIVATE_CONTRACT", `{"did":"did:contract:1"}`))

	require.Len(t, changes, 1)
	require.Equal(t, Change{DID: "did:contract:1", EventType: "ACTIVATE_CONTRACT"}, <-changes)
}

// CWE-9 (DCS-FR-CWE-24): a change that arrives from the counterparty instance
// (a shipped offer or counteroffer, a negotiation message, a termination) is
// recorded as a peer exchange and reaches the dashboards.
func TestAReceivedPeerExchangeIsStreamed(t *testing.T) {
	feed := New()
	changes, stop := feed.Subscribe()
	defer stop()

	feed.Handle(busEvent(t, "PEER_EXCHANGE", `{"did":"did:contract:1","exchange":"post_negotiation","direction":"RECEIVED"}`))

	require.Len(t, changes, 1)
	require.Equal(t, Change{DID: "did:contract:1", EventType: "PEER_EXCHANGE"}, <-changes)
}
