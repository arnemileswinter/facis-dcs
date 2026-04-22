package ocmw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"digital-contracting-service/internal/base/event"

	cloudevents "github.com/cloudevents/sdk-go/v2/event"
	"goa.design/clue/log"
)

// storageStoreMessage mirrors the subset of
// storage-service/pkg/messaging.StorageServiceStoreMessage we need.
//
// CV publishes one of these on the storage NATS topic after successfully
// verifying a wallet presentation (see CV's proofHandling.forwardPresentation).
// The Payload contains the raw verified VP (JSON-LD for LDP-VP).
type storageStoreMessage struct {
	TenantId    string `json:"tenant_id"`
	RequestId   string `json:"request_id"`
	GroupId     string `json:"group_id,omitempty"`
	AccountId   string `json:"accountId"`
	Type        string `json:"type"`
	Payload     []byte `json:"payload"`
	ContentType string `json:"contentType"`
	Id          string `json:"id"`
}

// storage-service constants (copied to avoid pulling the OCM-W library).
const (
	storagePresentationTopicDefault = "storage"
	storagePresentationType         = "storage.service.presentation"
)

// PresentationCompletedHandler is invoked when CV publishes a verified VP
// for a known DCS presentation request.
type PresentationCompletedHandler func(ctx context.Context, requestID string, vp []byte)

// StartPresentationStorageListener subscribes to the storage topic that
// CV publishes verified presentations on. It invokes onPresentation for
// every "storage.service.presentation" event so the auth service can
// finish the OAuth flow (Hydra login/consent accept) and mark the
// presentation request completed.
//
// The listener runs until ctx is cancelled.
func StartPresentationStorageListener(ctx context.Context, natsURL string, onPresentation PresentationCompletedHandler) error {
	topic := strings.TrimSpace(os.Getenv("CV_STORAGE_TOPIC"))
	if topic == "" {
		topic = storagePresentationTopicDefault
	}

	sub, err := event.NewNatsSubClient(topic, natsURL)
	if err != nil {
		return fmt.Errorf("failed to create storage subscriber: %w", err)
	}

	go func() {
		err := sub.Subscribe(func(evt cloudevents.Event) {
			if evt.Type() != storagePresentationType {
				return
			}
			var msg storageStoreMessage
			if err := json.Unmarshal(evt.Data(), &msg); err != nil {
				log.Errorf(ctx, err, "presentation-storage: invalid payload (type=%s)", evt.Type())
				return
			}
			if msg.RequestId == "" || len(msg.Payload) == 0 {
				return
			}
			log.Printf(ctx, "presentation-storage: received VP request_id=%s tenant=%s", msg.RequestId, msg.TenantId)
			onPresentation(ctx, msg.RequestId, msg.Payload)
		})
		if err != nil {
			log.Errorf(ctx, err, "presentation-storage: subscriber stopped")
		}
	}()

	go func() {
		<-ctx.Done()
		_ = sub.Close()
	}()

	return nil
}
