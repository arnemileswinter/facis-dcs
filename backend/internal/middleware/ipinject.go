// middleware/ip.go
package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type contextKey string

const IPKey contextKey = "client_ip"

func InjectIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		ctx := context.WithValue(r.Context(), IPKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func IPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(IPKey).(string)
	return ip
}

// extractIP returns the address the ingress saw the request come from: the
// right-most X-Forwarded-For entry, which the ingress appends. Entries to its
// left are sent by the client and can name any address, so they must not
// select whose failed attempts count toward a lockout (FR-UC-01-4).
// ponytail: one proxy hop (the ingress) is assumed; a chain of several
// proxies needs a list of trusted proxy addresses to skip.
func extractIP(r *http.Request) string {
	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		last := xff[len(xff)-1]
		if i := strings.LastIndex(last, ","); i >= 0 {
			last = last[i+1:]
		}
		if ip := strings.TrimSpace(last); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
