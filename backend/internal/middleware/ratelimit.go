package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

// RateLimitAuthenticated enforces the per-credential request budget for API
// interactions (DCS-FR-CWE-28): every request presenting an Authorization
// header is counted against a fixed one-minute window keyed by that
// credential, and requests beyond limitPerMinute are answered 429 with a
// Retry-After. Requests without an Authorization header are not counted —
// unauthenticated routes (login, DID document, static assets, probes) are
// not "API interactions" in the CWE-28 sense, and API routes reject them via
// their own auth anyway.
//
// The counters are in Postgres, so all replicas of the instance share one
// budget per credential. When the counter cannot be read, the request is
// let through: the limiter does not make a database fault a 429.
func RateLimitAuthenticated(db *sqlx.DB, limitPerMinute int, next http.Handler) http.Handler {
	l := &rateLimiter{db: db, limit: limitPerMinute}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			next.ServeHTTP(w, r)
			return
		}
		retryAfter, ok, err := l.take(r.Context(), auth, time.Now())
		if err != nil {
			log.Printf("rate limit: count request: %v", err)
		} else if !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
			http.Error(w, fmt.Sprintf("rate limit exceeded: more than %d API requests per minute for this credential", l.limit), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type rateLimiter struct {
	db    *sqlx.DB
	limit int
}

// take counts one request for the credential in one atomic upsert and reports
// whether it is within budget; when over budget it returns how long until the
// window resets. A window older than a minute restarts at now with count 1.
// When a window restarts, rows of windows that ended are deleted.
func (l *rateLimiter) take(ctx context.Context, credential string, now time.Time) (time.Duration, bool, error) {
	sum := sha256.Sum256([]byte(credential))
	var row struct {
		Start time.Time `db:"window_start"`
		Count int       `db:"request_count"`
	}
	err := l.db.QueryRowxContext(ctx, `
		INSERT INTO api_rate_limit_windows AS w (credential_hash, window_start, request_count)
		VALUES ($1, $2::timestamptz, 1)
		ON CONFLICT (credential_hash) DO UPDATE SET
			window_start  = CASE WHEN w.window_start <= $2 - INTERVAL '1 minute' THEN $2 ELSE w.window_start END,
			request_count = CASE WHEN w.window_start <= $2 - INTERVAL '1 minute' THEN 1 ELSE w.request_count + 1 END
		RETURNING window_start, request_count`, sum[:16], now).StructScan(&row)
	if err != nil {
		return 0, false, err
	}
	if row.Count == 1 {
		if _, err := l.db.ExecContext(ctx, `DELETE FROM api_rate_limit_windows WHERE window_start <= $1::timestamptz - INTERVAL '1 minute'`, now); err != nil {
			log.Printf("rate limit: delete ended windows: %v", err)
		}
	}
	if row.Count > l.limit {
		return time.Minute - now.Sub(row.Start), false, nil
	}
	return 0, true, nil
}
