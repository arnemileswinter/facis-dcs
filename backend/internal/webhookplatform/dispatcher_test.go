package webhookplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
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
	d := NewDispatcher(&fakeSubscriptions{}, NewDeliveryLog())
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
