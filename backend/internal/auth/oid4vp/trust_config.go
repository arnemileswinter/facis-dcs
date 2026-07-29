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

	// ORCEResolverURL is the flow endpoint the orce mechanism delegates to
	// (OID4VP_ORCE_RESOLVER_URL). Empty unless a deployment uses it.
	ORCEResolverURL string `json:"orce_resolver_url"`

	keyFetcher KeyFetcher
}

// Purpose is what an issuer's credentials may be used FOR. Trusting an issuer's
// signature says a credential is authentic; it does not say its holder may act
// here. Keeping the two apart is what lets an instance verify a counterparty's
// Power of Attorney without also accepting it as a login (ADR-31).
type Purpose string

const (
	// PurposeLogin: credentials from this issuer may grant a session here.
	PurposeLogin Purpose = "login"
	// PurposePeer: credentials from this issuer are verified when they arrive
	// from a counterparty, and when this instance presents its own side of a
	// mutual Power-of-Attorney binding.
	PurposePeer Purpose = "peer"
	// PurposePID: credentials from this issuer attest the identity of a natural
	// person. A PID is a third party's attestation — an instance that issued it
	// to itself has attested nothing.
	PurposePID Purpose = "pid"
)

func knownPurpose(p Purpose) bool {
	switch p {
	case PurposeLogin, PurposePeer, PurposePID:
		return true
	}
	return false
}

// Mechanism names how an issuer's verification key is resolved. Deployments
// differ in how issuers publish keys, and the production model is not yet
// settled, so this is configuration rather than a compiled-in assumption.
type Mechanism string

const (
	MechanismJWKS   Mechanism = "jwks"    // keys bundled in the trust entry
	MechanismX5C    Mechanism = "x5c"     // chain in the credential header, verified to configured roots
	MechanismDIDJWK Mechanism = "did:jwk" // key decoded from the issuer identifier
	MechanismDIDWeb Mechanism = "did:web" // key fetched from the issuer's DID document
	MechanismORCE   Mechanism = "orce"    // delegated to a configured ORCE flow
)

// supportedMechanisms are the ones this build can resolve. A deployment naming
// anything else is refused AT LOAD rather than at first use, so an unsupported
// trust configuration surfaces on startup instead of when a wallet arrives.
//
// A scheme absent from this list — did:ebsi, a national registry, whatever
// comes next — is reached through MechanismORCE without a change here: the flow
// resolves it and answers with a JWKS.
var supportedMechanisms = map[Mechanism]bool{
	MechanismJWKS:   true,
	MechanismX5C:    true,
	MechanismDIDJWK: true,
	MechanismDIDWeb: true,
	MechanismORCE:   true,
}

// TrustedIssuer is one issuer entry: what it may be trusted for, which
// organizations it may speak for, and how its key is resolved.
type TrustedIssuer struct {
	Purposes []Purpose `json:"purposes"`
	// Organizations bounds what this issuer may attest. A credential naming an
	// organization absent from this list is refused even when the signature is
	// good — otherwise any trusted issuer could speak for any party, and an
	// organization check at the callsite would depend on every issuer being
	// well-behaved rather than on something the verifier enforces.
	// Not required for a pid issuer: a PID attests a person, not a party.
	Organizations []string        `json:"organizations"`
	Mechanism     Mechanism       `json:"mechanism"`
	JWKS          json.RawMessage `json:"jwks"`
}

// Allows reports whether this issuer was granted the purpose.
func (t TrustedIssuer) Allows(p Purpose) bool {
	for _, granted := range t.Purposes {
		if granted == p {
			return true
		}
	}
	return false
}

// OrganizationsAny is the explicit wildcard: this issuer is authoritative for
// whichever organization it names. It suits an issuer that IS the tenant
// registry for its deployment — it knows its organizations, the verifier does
// not, and enumerating them in trust configuration would mean editing that file
// whenever a tenant is onboarded.
//
// It has to be written out. Treating an absent list as "any" is how an issuer
// silently gains the right to speak for a party nobody granted it.
const OrganizationsAny = "*"

// MayAttest reports whether this issuer was entitled to name the organization.
// An issuer with no organizations may attest none: the empty case fails closed.
func (t TrustedIssuer) MayAttest(org string) bool {
	org = strings.TrimSpace(org)
	if org == "" {
		return false
	}
	for _, allowed := range t.Organizations {
		allowed = strings.TrimSpace(allowed)
		if allowed == OrganizationsAny || allowed == org {
			return true
		}
	}
	return false
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

	for iss, entry := range cfg.Issuers {
		if len(entry.Purposes) == 0 {
			return nil, fmt.Errorf("trust config %q: issuer %q declares no purposes; an entry without purposes would have to mean either none or all, and defaulting to all is how a peer's issuer silently becomes a login issuer", path, iss)
		}
		for _, p := range entry.Purposes {
			if !knownPurpose(p) {
				return nil, fmt.Errorf("trust config %q: issuer %q declares unknown purpose %q", path, iss, p)
			}
		}
		if entry.Mechanism == "" {
			return nil, fmt.Errorf("trust config %q: issuer %q declares no mechanism", path, iss)
		}
		if !supportedMechanisms[entry.Mechanism] {
			return nil, fmt.Errorf("trust config %q: issuer %q declares mechanism %q, which this build cannot resolve", path, iss, entry.Mechanism)
		}
		if entry.Mechanism == MechanismJWKS && len(entry.JWKS) == 0 {
			return nil, fmt.Errorf("trust config %q: issuer %q uses mechanism jwks but bundles no keys", path, iss)
		}
		if entry.Mechanism == MechanismDIDJWK && !strings.HasPrefix(iss, "did:jwk:") {
			return nil, fmt.Errorf("trust config %q: issuer %q uses mechanism did:jwk but is not a did:jwk identifier", path, iss)
		}
		if entry.Mechanism == MechanismDIDWeb && !strings.HasPrefix(iss, "did:web:") {
			return nil, fmt.Errorf("trust config %q: issuer %q uses mechanism did:web but is not a did:web identifier", path, iss)
		}
		// A pid issuer attests a person, not a party, so it needs no
		// organizations. Anything that can speak for a party must say which.
		if !entry.Allows(PurposePID) && len(entry.Organizations) == 0 {
			return nil, fmt.Errorf("trust config %q: issuer %q may act for a party but lists no organizations", path, iss)
		}
	}

	return &cfg, nil
}

// For returns a view of this configuration restricted to one purpose. Key
// resolution (sdjwt) asks only "is this issuer trusted?", so scoping happens by
// handing it a view that answers that question for the purpose at hand.
func (c *TrustConfig) For(p Purpose) *PurposeView { return &PurposeView{cfg: c, purpose: p} }

// PurposeView is a TrustConfig restricted to a single purpose.
type PurposeView struct {
	cfg     *TrustConfig
	purpose Purpose
}

func (v *PurposeView) IssuerTrusted(iss string) bool {
	if v == nil || v.cfg == nil {
		return false
	}
	entry, ok := v.cfg.Issuers[strings.TrimSpace(iss)]
	return ok && entry.Allows(v.purpose)
}

func (v *PurposeView) VCTAllowed(vct string) bool { return v.cfg.VCTAllowed(vct) }

func (v *PurposeView) IssuerJWKS(iss string) (json.RawMessage, error) {
	if !v.IssuerTrusted(iss) {
		return nil, fmt.Errorf("issuer %q is not trusted for %s", iss, v.purpose)
	}
	return v.cfg.resolveIssuerKeys(iss)
}

func (v *PurposeView) X5CTrustRoots() *x509.CertPool { return v.cfg.X5CTrustRoots() }

// IssuerMayAttest reports whether the issuer was entitled to name this
// organization.
func (c *TrustConfig) IssuerMayAttest(iss, org string) bool {
	if c == nil {
		return false
	}
	entry, ok := c.Issuers[strings.TrimSpace(iss)]
	return ok && entry.MayAttest(org)
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
