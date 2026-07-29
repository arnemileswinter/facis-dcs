// Package safehttp builds HTTP clients for fetches whose URL is derived from
// data a caller supplies — a did:web identifier off an unauthenticated wallet
// callback, an ORCE resolver path. Such a fetch is a server-side request
// primitive: without constraints it reads whatever the service itself can
// reach, and its result is then trusted as key material.
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Policy bounds where a client may connect.
//
// Private ranges stay reachable by default because that is where these
// deployments genuinely live: an in-cluster ORCE resolver and a peer DCS are
// Service addresses. Blocking them wholesale would break the normal case while
// an attacker who can name a public host is unaffected. AllowedHosts is the
// control for a deployment that wants the tighter bound, and the primary defence
// remains that only issuers named in the trust configuration are ever resolved.
type Policy struct {
	// AllowedHosts, when non-empty, is the exhaustive set of hostnames that may
	// be dialled. Compared case-insensitively against the URL host, port excluded.
	AllowedHosts []string
	// AllowLoopback permits 127.0.0.0/8 and ::1, which dev and CI stacks need and
	// a real deployment never does — loopback there is the service's own
	// admin surface.
	AllowLoopback bool
}

// Client returns an HTTP client that refuses redirects and applies p to every
// address it dials.
func Client(timeout time.Duration, p Policy) *http.Client {
	allowed := make(map[string]bool, len(p.AllowedHosts))
	for _, h := range p.AllowedHosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			allowed[h] = true
		}
	}

	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("dial %s: %w", addr, err)
			}
			if len(allowed) > 0 && !allowed[strings.ToLower(host)] {
				return nil, fmt.Errorf("dial %s: host is not in the resolver allow-list", host)
			}
			// Resolution happens here rather than being left to the dialer so the
			// address that gets connected to is the address that was checked. A
			// name checked and then re-resolved by the dialer can answer
			// differently the second time.
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", host, err)
			}
			for _, ip := range ips {
				if err := permitted(ip.IP, p.AllowLoopback); err != nil {
					return nil, fmt.Errorf("dial %s: %w", host, err)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// A DID document and a JWKS are served, not redirected to. Following a
		// redirect would let the responder pick the next address after the first
		// was checked, and would silently undo the https-only rule by sending the
		// second hop to http.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("refusing redirect to %s: key material is fetched from the address the identifier names, not one it is pointed at", req.URL.Redacted())
		},
	}
}

// permitted rejects addresses that no legitimate issuer or peer is published on
// and that a request forgery aims at.
func permitted(ip net.IP, allowLoopback bool) error {
	switch {
	case ip.IsUnspecified():
		return fmt.Errorf("address %s is unspecified", ip)
	case ip.IsLoopback():
		if allowLoopback {
			return nil
		}
		return fmt.Errorf("address %s is loopback", ip)
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// 169.254.169.254 and fe80:: neighbours: the cloud instance metadata
		// service and anything else that answers only because the request comes
		// from this host.
		return fmt.Errorf("address %s is link-local", ip)
	case ip.IsMulticast():
		return fmt.Errorf("address %s is multicast", ip)
	case ip.IsInterfaceLocalMulticast():
		return fmt.Errorf("address %s is interface-local", ip)
	}
	return nil
}
