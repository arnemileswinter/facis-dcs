package oid4vp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"digital-contracting-service/internal/auth/oid4vp/sdjwt"
	"digital-contracting-service/internal/base/safehttp"
)

// KeyFetcher retrieves a document over HTTP. Injected so did:web and the ORCE
// delegation are testable without a network, and so a deployment can supply its
// own transport.
type KeyFetcher interface {
	Fetch(url string) ([]byte, error)
}

type httpFetcher struct{ client *http.Client }

func (f httpFetcher) Fetch(url string) ([]byte, error) {
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}
	return body, nil
}

// DefaultKeyFetcher is the transport used when a deployment configures no other.
//
// The URL these fetches reach is derived from an identifier that arrives with a
// credential, so it is bounded rather than free: no redirects, and no dialling
// the addresses that answer only because the request originates here. Set
// OID4VP_RESOLVER_ALLOWED_HOSTS to reduce it further to a named set.
func DefaultKeyFetcher() KeyFetcher {
	return httpFetcher{client: safehttp.Client(10*time.Second, resolverPolicy())}
}

func resolverPolicy() safehttp.Policy {
	var hosts []string
	for _, h := range strings.Split(os.Getenv("OID4VP_RESOLVER_ALLOWED_HOSTS"), ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	return safehttp.Policy{
		AllowedHosts: hosts,
		// Dev and CI stacks publish issuers and peers on localhost ports; a
		// deployment that does is pointing the resolver at its own admin surface.
		AllowLoopback: os.Getenv("OID4VP_RESOLVER_ALLOW_LOOPBACK") == "true",
	}
}

// resolveIssuerKeys produces the JWKS an issuer's signature is verified
// against, by the mechanism its trust entry declares.
//
// An x5c issuer resolves to no JWKS by design: its key arrives in the
// credential's own certificate chain and is verified against the configured
// roots, so there is nothing to look up here. Returning empty rather than an
// error lets the chain path run; a credential from that issuer bearing a bare
// jwk header then finds nothing to match and is refused, which is correct — an
// issuer that publishes via certificates has not published a bare key.
func (c *TrustConfig) resolveIssuerKeys(iss string) (json.RawMessage, error) {
	iss = strings.TrimSpace(iss)
	entry, ok := c.Issuers[iss]
	if !ok {
		// A peer trusted dynamically has no entry by design; its key comes from
		// its own DID document, which is the source of truth for it.
		if c.dynamicPeerIssuer(PurposePeer, iss) {
			return c.jwksFromDIDWeb(iss)
		}
		return nil, fmt.Errorf("issuer %q is not trusted", iss)
	}

	switch entry.Mechanism {
	case MechanismJWKS:
		if len(entry.JWKS) == 0 {
			return nil, fmt.Errorf("issuer %q has no jwks", iss)
		}
		return entry.JWKS, nil

	case MechanismX5C:
		return nil, nil

	case MechanismDIDJWK:
		return jwksFromDIDJWK(iss)

	case MechanismDIDWeb:
		return c.jwksFromDIDWeb(iss)

	case MechanismORCE:
		return c.jwksFromORCE(iss)
	}

	return nil, fmt.Errorf("issuer %q declares mechanism %q, which this build cannot resolve", iss, entry.Mechanism)
}

// jwksFromDIDJWK reads the key out of the identifier itself. No I/O and nothing
// to trust beyond the identifier: a did:jwk IS its key, so an issuer named this
// way cannot rotate without becoming a different issuer.
func jwksFromDIDJWK(iss string) (json.RawMessage, error) {
	key, err := sdjwt.JWKFromDIDJWK(iss)
	if err != nil {
		return nil, fmt.Errorf("issuer %q: %w", iss, err)
	}
	return marshalJWKS(key)
}

// didWebURL maps did:web:host:a:b to https://host/a/b/did.json, per the did:web
// method (a port arrives percent-encoded in the identifier).
func didWebURL(iss string) (string, error) {
	rest := strings.TrimPrefix(iss, "did:web:")
	if rest == "" || rest == iss {
		return "", fmt.Errorf("issuer %q is not a did:web identifier", iss)
	}
	segments := strings.Split(rest, ":")
	authority := strings.ReplaceAll(segments[0], "%3A", ":")
	path := strings.Join(segments[1:], "/")
	if path == "" {
		return "https://" + authority + "/.well-known/did.json", nil
	}
	return "https://" + authority + "/" + path + "/did.json", nil
}

func (c *TrustConfig) jwksFromDIDWeb(iss string) (json.RawMessage, error) {
	url, err := didWebURL(iss)
	if err != nil {
		return nil, err
	}
	body, err := c.fetcher().Fetch(url)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", iss, err)
	}

	// assertionMethod says which keys may make assertions. A DID document
	// publishes that separation deliberately — our own gendid puts the ECDH
	// key-agreement key in the same document — so a resolver that collects every
	// verification method lets a key meant for encryption verify signatures.
	var doc struct {
		ID                 string `json:"id"`
		AssertionMethod    []any  `json:"assertionMethod"`
		VerificationMethod []struct {
			ID           string    `json:"id"`
			PublicKeyJWK sdjwt.JWK `json:"publicKeyJwk"`
		} `json:"verificationMethod"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse did document for %s: %w", iss, err)
	}
	if len(doc.VerificationMethod) == 0 {
		return nil, fmt.Errorf("did document for %s carries no verification method", iss)
	}
	// A document that identifies itself as somebody else says nothing about the
	// issuer we asked about, however it was reached.
	if strings.TrimSpace(doc.ID) != iss {
		return nil, fmt.Errorf("did document at %s identifies itself as %q, not %q", url, doc.ID, iss)
	}

	assertion := map[string]bool{}
	for _, entry := range doc.AssertionMethod {
		switch v := entry.(type) {
		case string:
			assertion[v] = true
		case map[string]any:
			if id, ok := v["id"].(string); ok {
				assertion[id] = true
			}
		}
	}
	if len(assertion) == 0 {
		return nil, fmt.Errorf("did document for %s lists no assertionMethod, so none of its keys may make assertions", iss)
	}

	keys := make([]sdjwt.JWK, 0, len(doc.VerificationMethod))
	for _, vm := range doc.VerificationMethod {
		if vm.PublicKeyJWK.X == "" || !assertion[vm.ID] {
			continue
		}
		key := vm.PublicKeyJWK
		// The credential names the verification method in its kid, so carry the
		// id across or nothing matches it.
		if key.Kid == "" {
			key.Kid = vm.ID
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("did document for %s has no assertionMethod key usable for verification", iss)
	}
	return marshalJWKS(keys...)
}

// jwksFromORCE delegates resolution to a configured ORCE flow. This is the
// escape hatch the architecture rests on: a registry this build knows nothing
// about — did:ebsi today, something else later — becomes reachable by pointing
// an issuer at a flow that returns its keys, with no change here.
func (c *TrustConfig) jwksFromORCE(iss string) (json.RawMessage, error) {
	base := strings.TrimSpace(c.ORCEResolverURL)
	if base == "" {
		return nil, fmt.Errorf("issuer %q uses the orce mechanism but no resolver endpoint is configured (OID4VP_ORCE_RESOLVER_URL)", iss)
	}
	body, err := c.fetcher().Fetch(strings.TrimSuffix(base, "/") + "/" + iss)
	if err != nil {
		return nil, fmt.Errorf("resolve %s via orce: %w", iss, err)
	}

	// The flow answers with a JWKS; anything else is a misconfigured flow
	// rather than an untrusted issuer, and saying so plainly saves an hour.
	var probe struct {
		Keys []sdjwt.JWK `json:"keys"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("orce resolver returned no parseable jwks for %s: %w", iss, err)
	}
	if len(probe.Keys) == 0 {
		return nil, fmt.Errorf("orce resolver returned no keys for %s", iss)
	}
	return body, nil
}

func marshalJWKS(keys ...sdjwt.JWK) (json.RawMessage, error) {
	raw, err := json.Marshal(struct {
		Keys []sdjwt.JWK `json:"keys"`
	}{Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("marshal jwks: %w", err)
	}
	return raw, nil
}

func (c *TrustConfig) fetcher() KeyFetcher {
	if c.keyFetcher == nil {
		return DefaultKeyFetcher()
	}
	return c.keyFetcher
}

// SetKeyFetcher installs the transport used by the did:web and orce
// mechanisms.
func (c *TrustConfig) SetKeyFetcher(f KeyFetcher) { c.keyFetcher = f }

// issuerUsesX5C reports whether the issuer publishes its key through a
// certificate chain. A dynamically trusted peer does not: it publishes a DID
// document, which is what makes it resolvable without an entry.
func (c *TrustConfig) issuerUsesX5C(iss string) (bool, error) {
	iss = strings.TrimSpace(iss)
	entry, ok := c.Issuers[iss]
	if !ok {
		if c.dynamicPeerIssuer(PurposePeer, iss) {
			return false, nil
		}
		return false, fmt.Errorf("issuer %q is not trusted", iss)
	}
	return entry.Mechanism == MechanismX5C, nil
}
