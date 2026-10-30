package webhookplatform

import (
	"context"
	"sync"
)

// memDeliveries is the in-memory Deliveries the unit tests use.
type memDeliveries struct {
	mu         sync.Mutex
	pending    map[string]PendingCallback
	deliveries []Delivery
}

func newMemDeliveries() *memDeliveries {
	return &memDeliveries{pending: make(map[string]PendingCallback)}
}

func (s *memDeliveries) TrackPending(_ context.Context, p PendingCallback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[p.EventID] = p
	return nil
}

func (s *memDeliveries) ResolvePending(_ context.Context, eventID string) (PendingCallback, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[eventID]
	delete(s.pending, eventID)
	return p, ok, nil
}

func (s *memDeliveries) AddDelivery(_ context.Context, d Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, d)
	return nil
}

func (s *memDeliveries) ListDeliveries(_ context.Context, ownerDID string) ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Delivery{}
	for _, d := range s.deliveries {
		if d.OwnerDID == ownerDID {
			out = append(out, d)
		}
	}
	return out, nil
}
