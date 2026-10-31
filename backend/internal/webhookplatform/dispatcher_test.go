package webhookplatform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	cloudevent "github.com/cloudevents/sdk-go/v2/event"
)

func TestDeliveryIsSignedOverTheSentBody(t *testing.T) {
	var got struct {
		header http.Header
		body   []byte
	}
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.header = r.Header.Clone()
		got.body, _ = io.ReadAll(r.Body)
	}))
	defer receiver.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher(&fakeSubscriptions{}, newMemDeliveries(), admitAll, nil)
	d.httpClient = receiver.Client()
	payload := WebhookPayload{
		EventID:       "evt-1",
		CorrelationID: "corr-1",
		Event:         "template.updated",
		DID:           "did:web:org-a.example:template:1",
		OccurredAt:    time.Now().UTC(),
		Data:          json.RawMessage(`{"name":"a<b&c","version":1.0}`),
	}
	d.notify(Subscription{CallbackURL: receiver.URL, Secret: secret}, payload)

	if auth := got.header.Get("Authorization"); auth != "" {
		t.Fatalf("delivery carries Authorization %q", auth)
	}
	if id := got.header.Get("webhook-id"); id != payload.EventID {
		t.Fatalf("webhook-id = %q, want %q", id, payload.EventID)
	}
	timestamp := got.header.Get("webhook-timestamp")
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)).Abs() > time.Minute {
		t.Fatalf("webhook-timestamp = %q, want the current unix time in seconds", timestamp)
	}
	want, err := Signature(secret, payload.EventID, timestamp, got.body)
	if err != nil || got.header.Get("webhook-signature") != want {
		t.Fatalf("webhook-signature = %q, want %q over the received body", got.header.Get("webhook-signature"), want)
	}
}

// Contract events carry the contract content: only organizations the audience
// admits receive them.
func TestDispatchDeliversOnlyToAdmittedOrganizations(t *testing.T) {
	hits := make(chan string, 2)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- r.URL.Path
	}))
	defer receiver.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	subs := &fakeSubscriptions{subs: []Subscription{
		{ID: "a", Event: "contract.created", CallbackURL: receiver.URL + "/a", Secret: secret, OwnerDID: orgA},
		{ID: "b", Event: "contract.created", CallbackURL: receiver.URL + "/b", Secret: secret, OwnerDID: orgB},
	}}
	audience := func(_ context.Context, event, did, owner string) bool {
		return event == "contract.created" && did == "contract-1" && owner == orgA
	}
	d := NewDispatcher(subs, newMemDeliveries(), audience, nil)
	d.httpClient = receiver.Client()

	if err := d.Dispatch(context.Background(), "event-1", "contract.created", "contract-1", json.RawMessage(`{"contract_data":{}}`)); err != nil {
		t.Fatal(err)
	}

	select {
	case path := <-hits:
		if path != "/a" {
			t.Fatalf("delivered to %s, want only /a", path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no delivery to the admitted organization")
	}
	select {
	case path := <-hits:
		t.Fatalf("second delivery to %s: org B is not admitted", path)
	case <-time.After(300 * time.Millisecond):
	}
}

// DCS-NFR-SF-01: JetStream delivers an event at least once. A redelivered
// event has the same CloudEvent id, which is the webhook event id, and is not
// sent to the subscriber a second time.
func TestRedeliveredEventIsDispatchedOnce(t *testing.T) {
	hits := make(chan string, 2)
	receiver := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		hits <- r.Header.Get("webhook-id")
	}))
	defer receiver.Close()
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	subs := &fakeSubscriptions{subs: []Subscription{
		{ID: "a", Event: "contract.created", CallbackURL: receiver.URL, Secret: secret, OwnerDID: orgA},
	}}
	d := NewDispatcher(subs, newMemDeliveries(), admitAll, nil)
	d.httpClient = receiver.Client()

	evt := cloudevent.New()
	evt.SetID("event-7")
	evt.SetSource("CONTRACT_WORKFLOW_ENGINE")
	evt.SetType("CREATE_CONTRACT")
	if err := evt.SetData(cloudevent.ApplicationJSON, json.RawMessage(`{"did":"contract-1"}`)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := handleNATSEvent(context.Background(), evt, d); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case id := <-hits:
		if id != "event-7" {
			t.Fatalf("webhook-id %q, want the CloudEvent id", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no delivery")
	}
	select {
	case <-hits:
		t.Fatal("the redelivered event was sent again")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestDefaultClientRefusesLoopbackCallbacks(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the dispatcher reached a loopback callback")
	}))
	defer receiver.Close()
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	store := newMemDeliveries()
	d := NewDispatcher(&fakeSubscriptions{}, store, admitAll, nil)
	d.notify(Subscription{CallbackURL: receiver.URL, Secret: secret, OwnerDID: orgA}, WebhookPayload{EventID: "e", Event: "template.updated"})
	got, _ := store.ListDeliveries(context.Background(), orgA)
	if len(got) != 1 || got[0].Error == "" || got[0].StatusCode != 0 {
		t.Fatalf("deliveries = %+v, want one refused delivery", got)
	}
}
