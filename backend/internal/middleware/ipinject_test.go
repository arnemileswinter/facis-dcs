package middleware

import (
	"net/http/httptest"
	"testing"
)

// A client cannot choose the address its attempts are counted under by
// sending its own X-Forwarded-For: only the hop the ingress appends counts.
func TestExtractIPTakesTheHopTheIngressAppended(t *testing.T) {
	for name, tc := range map[string]struct {
		forwarded []string
		want      string
	}{
		"client sent a spoofed entry":       {[]string{"198.51.100.7, 203.0.113.9"}, "203.0.113.9"},
		"ingress appended a second header":  {[]string{"198.51.100.7", "203.0.113.9"}, "203.0.113.9"},
		"single entry":                      {[]string{"203.0.113.9"}, "203.0.113.9"},
		"no header uses the remote address": {nil, "192.0.2.1"},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "192.0.2.1:4711"
			for _, v := range tc.forwarded {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := extractIP(r); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
