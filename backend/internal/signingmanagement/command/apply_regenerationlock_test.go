package command

import (
	"errors"
	"strings"
	"testing"

	"github.com/lib/pq"
)

// A lock wait cut short by lock_timeout means the background regenerator still
// holds the contract. Reporting it as ErrRegenerationInFlight is what keeps the
// caller from reading a generic database failure into contention that resolves
// on its own — the condition the whole bounded wait exists to surface.
func TestRegenerationLockErrorNamesContention(t *testing.T) {
	err := regenerationLockError("did:contract:1", &pq.Error{Code: pqLockNotAvailable, Message: "canceling statement due to lock timeout"})

	if !errors.Is(err, ErrRegenerationInFlight) {
		t.Fatalf("a lock_timeout must report ErrRegenerationInFlight, got %v", err)
	}
	if !strings.Contains(err.Error(), "did:contract:1") {
		t.Fatalf("the contract must be named, got %v", err)
	}
}

// Any other failure of the lock statement is a database failure, not
// contention: reporting it as retry-later would tell the caller to wait out a
// condition that is not going to clear.
func TestRegenerationLockErrorKeepsOtherFailuresDistinct(t *testing.T) {
	for name, cause := range map[string]error{
		"other postgres error": &pq.Error{Code: "42P01", Message: "relation does not exist"},
		"transport failure":    errors.New("connection reset by peer"),
	} {
		t.Run(name, func(t *testing.T) {
			err := regenerationLockError("did:contract:1", cause)

			if errors.Is(err, ErrRegenerationInFlight) {
				t.Fatalf("%v must not be reported as regeneration contention", cause)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("the cause must be preserved, got %v", err)
			}
		})
	}
}
