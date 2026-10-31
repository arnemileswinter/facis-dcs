package db

import (
	"context"
	"testing"

	"digital-contracting-service/internal/base/pgtest"
)

// DCS-NFR-PER-02: a scheduled job runs on one replica at a time. While one
// replica runs it, the other replica skips; after it returns, the job is free.
func TestRunExclusiveRunsAJobOnOneReplicaAtATime(t *testing.T) {
	database := pgtest.Open(t)
	ctx := context.Background()

	var otherRan, otherOK bool
	ran, err := RunExclusive(ctx, database, "test-job", func() error {
		ok, err := RunExclusive(ctx, database, "test-job", func() error { otherRan = true; return nil })
		otherOK = ok
		if err != nil {
			t.Errorf("second replica: %v", err)
		}
		// A different job is not blocked.
		if ok, err := RunExclusive(ctx, database, "other-job", func() error { return nil }); err != nil || !ok {
			t.Errorf("other job while test-job runs = %v, %v; want it to run", ok, err)
		}
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("first replica = %v, %v; want it to run", ran, err)
	}
	if otherOK || otherRan {
		t.Fatal("second replica ran the job while the first replica held it")
	}

	if ok, err := RunExclusive(ctx, database, "test-job", func() error { return nil }); err != nil || !ok {
		t.Fatalf("after the first replica finished = %v, %v; want the job free again", ok, err)
	}
}
