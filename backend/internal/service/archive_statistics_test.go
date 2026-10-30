package service

import "testing"

func TestAuditEntryActorLifecycleOperations(t *testing.T) {
	for _, raw := range []string{`{"stored_by":"alice"}`, `{"approved_by":"alice"}`, `{"signed_by":"alice"}`} {
		if got := auditEntryActor([]byte(raw)); got != "alice" {
			t.Fatalf("actor for %s = %q", raw, got)
		}
	}
	if got := auditEntryActor([]byte(`{"approved_by":"alice","signed_by":"bob"}`)); got != "" {
		t.Fatalf("ambiguous actor = %q", got)
	}
}
