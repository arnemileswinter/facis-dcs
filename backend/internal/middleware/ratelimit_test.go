package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"digital-contracting-service/internal/base/pgtest"
)

// DCS-FR-CWE-28 with more than one replica: two limiter instances over the
// same database enforce one budget per credential, not one budget each.
func TestRateLimitAuthenticatedEnforcesOneBudgetAcrossReplicas(t *testing.T) {
	db := pgtest.Open(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	replicas := []http.Handler{RateLimitAuthenticated(db, 3, next), RateLimitAuthenticated(db, 3, next)}

	do := func(replica int, token string) int {
		req := httptest.NewRequest(http.MethodGet, "/contract/search", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		replicas[replica].ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 3; i++ {
		if code := do(i%2, "alice"); code != http.StatusOK {
			t.Fatalf("request %d within budget got %d", i+1, code)
		}
	}
	for replica := range replicas {
		if code := do(replica, "alice"); code != http.StatusTooManyRequests {
			t.Fatalf("request over budget on replica %d got %d, want 429", replica, code)
		}
	}
	// A different credential has its own budget.
	if code := do(1, "bob"); code != http.StatusOK {
		t.Fatalf("other credential got %d, want 200", code)
	}
	// Unauthenticated requests are never counted or limited.
	for i := 0; i < 10; i++ {
		if code := do(i%2, ""); code != http.StatusOK {
			t.Fatalf("unauthenticated request got %d, want 200", code)
		}
	}
}

func TestRateLimiterWindowResets(t *testing.T) {
	db := pgtest.Open(t)
	l := &rateLimiter{db: db, limit: 1}
	ctx := context.Background()
	now := time.Now()
	if _, ok, err := l.take(ctx, "tok", now); err != nil || !ok {
		t.Fatalf("first take = %v, %v; want within budget", ok, err)
	}
	if retry, ok, err := l.take(ctx, "tok", now.Add(30*time.Second)); err != nil || ok || retry <= 0 || retry > 31*time.Second {
		t.Fatalf("second take in the same window = %v, %v, retry %v; want over budget, retry about 30s", ok, err, retry)
	}
	if _, ok, err := l.take(ctx, "tok", now.Add(61*time.Second)); err != nil || !ok {
		t.Fatalf("take after the window elapsed = %v, %v; want the budget reset", ok, err)
	}
}

// Concurrent requests for one credential on two replicas are each counted
// exactly once.
func TestRateLimiterCountsConcurrentRequestsAtomically(t *testing.T) {
	db := pgtest.Open(t)
	replicas := []*rateLimiter{{db: db, limit: 10}, {db: db, limit: 10}}
	now := time.Now()
	results := make(chan bool, 30)
	for i := 0; i < 30; i++ {
		go func(i int) {
			_, ok, err := replicas[i%2].take(context.Background(), "tok", now)
			results <- ok && err == nil
		}(i)
	}
	allowed := 0
	for i := 0; i < 30; i++ {
		if <-results {
			allowed++
		}
	}
	if allowed != 10 {
		t.Fatalf("allowed = %d of 30 concurrent requests, want exactly the budget of 10", allowed)
	}
}

// A new window deletes the rows of windows that ended, so the table does not
// grow with every credential ever seen.
func TestRateLimiterDeletesEndedWindows(t *testing.T) {
	db := pgtest.Open(t)
	l := &rateLimiter{db: db, limit: 10}
	ctx := context.Background()
	now := time.Now()
	if _, _, err := l.take(ctx, "old", now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.take(ctx, "new", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.Get(&n, `SELECT count(*) FROM api_rate_limit_windows`); err != nil || n != 1 {
		t.Fatalf("rows = %d, %v; want 1 (the ended window deleted)", n, err)
	}
}
