package identity

import (
	"crypto/tls"
	"net/http"
	"testing"
)

// The national trusted lists stay reachable on a server that offers TLS 1.2
// only, while every other outbound client requires TLS 1.3.
func TestTrustedListClientAcceptsTLS12(t *testing.T) {
	rt, ok := tslClient.Transport.(*http.Transport)
	if !ok || rt.TLSClientConfig == nil || rt.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("trusted-list transport %#v, want MinVersion TLS 1.2", tslClient.Transport)
	}
}

// UC-IF-28: peers are checked against the EU trusted lists unless the
// operator turns it off or the stack declares itself a dev stack.
func TestEUTrustPoolIsOnByDefault(t *testing.T) {
	for _, tc := range []struct {
		pool, devTrust string
		want           bool
	}{
		{"", "", true},
		{"", "true", false},
		{"false", "", false},
		{"true", "true", true},
	} {
		t.Setenv("DCS_EU_TRUST_POOL", tc.pool)
		t.Setenv("DCS_ALLOW_DEV_TRUST", tc.devTrust)
		if got := EUTrustPoolEnabled(); got != tc.want {
			t.Errorf("DCS_EU_TRUST_POOL=%q DCS_ALLOW_DEV_TRUST=%q: enabled = %t, want %t", tc.pool, tc.devTrust, got, tc.want)
		}
	}
}
