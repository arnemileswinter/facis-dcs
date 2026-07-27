package oid4vp

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TrustConfig is the verifier trust anchor loaded from trust.dev.json (OID4VP_TRUST_DATA_PATH).
// It records which credential types and issuer DIDs are accepted, and bundles their JWKS.
// JWT public-key resolution for issuer signatures is in sdjwt/keys.go.
type TrustConfig struct {
	VCTs    []string                 `json:"vcts"`
	Issuers map[string]TrustedIssuer `json:"issuers"`

	// x5cRoots are the trust anchors an x5c-bearing credential's certificate
	// chain must verify against (OID4VP_X5C_TRUST_ANCHORS_PATH) — a real
	// EUDI-wallet-issued PID carries its issuer certificate this way, not a
	// bare JWK. Optional: nil until a real wallet needs it (dev/BDD issue PID
	// credentials via the JWKS issuer path below, never x5c), but an x5c
	// credential presented with no roots configured must be REFUSED, never
	// silently trusted off its own embedded leaf cert.
	x5cRoots *x509.CertPool
}

// TrustedIssuer holds verification keys for one issuer DID entry in trust configuration.
type TrustedIssuer struct {
	JWKS json.RawMessage `json:"jwks"`
}

// LoadTrustConfig reads trust data from a JSON file (ConfigMap mount).
func LoadTrustConfig(path string) (*TrustConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("trust config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trust config %q: %w", path, err)
	}

	var cfg TrustConfig
	err = json.Unmarshal(data, &cfg)

	if err != nil {
		return nil, fmt.Errorf("parse trust config %q: %w", path, err)
	}

	if len(cfg.VCTs) == 0 {
		return nil, fmt.Errorf("trust config %q: vcts is required", path)
	}

	if len(cfg.Issuers) == 0 {
		return nil, fmt.Errorf("trust config %q: issuers is required", path)
	}

	return &cfg, nil
}

func (c *TrustConfig) IssuerTrusted(iss string) bool {
	if c == nil {
		return false
	}
	_, ok := c.Issuers[strings.TrimSpace(iss)]

	return ok
}

func (c *TrustConfig) VCTAllowed(vct string) bool {
	if c == nil {
		return false
	}

	vct = strings.TrimSpace(vct)

	for _, allowed := range c.VCTs {
		if vct == allowed {
			return true
		}
	}

	return false
}

func (c *TrustConfig) IssuerJWKS(iss string) (json.RawMessage, error) {
	entry, ok := c.Issuers[strings.TrimSpace(iss)]
	if !ok {
		return nil, fmt.Errorf("issuer %q is not trusted", iss)
	}

	if len(entry.JWKS) == 0 {
		return nil, fmt.Errorf("issuer %q has no jwks", iss)
	}

	return entry.JWKS, nil
}

// X5CTrustRoots returns the configured x5c chain-validation trust anchors, or
// nil if none were loaded (an x5c-bearing credential must then be refused,
// never accepted off its own embedded leaf cert — see sdjwt.verificationKeyFromX5C).
func (c *TrustConfig) X5CTrustRoots() *x509.CertPool {
	if c == nil {
		return nil
	}
	return c.x5cRoots
}

// SetX5CTrustRoots attaches the x5c chain-validation trust anchors loaded via
// LoadX5CTrustAnchors. Separate from LoadTrustConfig because the anchors are
// PEM certificates, not JSON, and are optional (OID4VP_X5C_TRUST_ANCHORS_PATH).
func (c *TrustConfig) SetX5CTrustRoots(pool *x509.CertPool) {
	c.x5cRoots = pool
}

// LoadX5CTrustAnchors reads a PEM bundle of one or more trusted root
// certificates (ConfigMap mount) that x5c-bearing credential chains must
// verify against.
func LoadX5CTrustAnchors(path string) (*x509.CertPool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("x5c trust anchors path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read x5c trust anchors %q: %w", path, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("x5c trust anchors %q: no valid PEM certificates found", path)
	}

	return pool, nil
}
