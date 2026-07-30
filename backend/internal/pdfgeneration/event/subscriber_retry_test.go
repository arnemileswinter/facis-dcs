package event

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The retry pass has no event to work from — the one that first asked for the
// regeneration was consumed and is not redelivered. It must therefore drive the
// handler off the DID alone, with an effective time of now, so the handler
// re-reads state and content from the record.
func TestRetryOneDrivesRegenerationFromTheDIDAlone(t *testing.T) {
	var seen minimalCWEEvent
	before := time.Now().UTC()

	s := &Subscriber{}
	s.retryOne("contract", "did:contract:1", func(_ context.Context, evt minimalCWEEvent) error {
		seen = evt
		return nil
	})

	if seen.DID != "did:contract:1" {
		t.Fatalf("the regenerated DID must be the one retried, got %q", seen.DID)
	}
	if seen.Reason != "" || seen.NewState != "" {
		t.Fatalf("a retry carries no event fields, got reason %q / state %q", seen.Reason, seen.NewState)
	}
	if seen.OccurredAt.Before(before) {
		t.Fatalf("the attempt must be effective now, got %s", seen.OccurredAt)
	}
}

// A retry that fails must not take the pass down with it: the next entity still
// gets its attempt, and the failed one is picked up again on the next tick
// because its record still shows no stored PDF.
func TestRetryOneSurvivesAFailedRegeneration(t *testing.T) {
	s := &Subscriber{}

	s.retryOne("template", "did:template:1", func(context.Context, minimalCWEEvent) error {
		return errors.New("artifact store unavailable")
	})
}

// The regeneration context carries a deadline, so a wedged pdf-core or artifact
// store cannot hold the regenerator open indefinitely.
func TestRegenerationContextIsBounded(t *testing.T) {
	s := &Subscriber{}
	ctx, cancel := s.regenerationContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("a regeneration attempt must run under a deadline")
	}
	if remaining := time.Until(deadline); remaining > regenerationTimeout {
		t.Fatalf("the deadline must be at most %s away, got %s", regenerationTimeout, remaining)
	}
}
