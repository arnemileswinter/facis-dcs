package oid4vp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTrust(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trust.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write trust config: %v", err)
	}
	return path
}

const jwksBlock = `{"keys":[{"kty":"EC","crv":"P-256","x":"sAYnZiIkBGJWkgViAZy4Jsdsp3DXnL1mV7hYQKJYKss","y":"0e6ZLeEnI57444v4hIXDEvZQVgnxjFtv8-4oLqls3_o"}]}`

// A peer's issuer must not be usable to mint a session here: that is the whole
// reason trust is scoped by purpose rather than being one boolean.
func TestIssuerTrustedForOnlyTheGrantedPurpose(t *testing.T) {
	path := writeTrust(t, `{
      "vcts": ["urn:dcs:poa:v1"],
      "issuers": {
        "https://own.example/issuer": {
          "purposes": ["login","peer"],
          "organizations": ["did:web:own.example"],
          "mechanism": "jwks",
          "jwks": `+jwksBlock+`
        },
        "https://peer.example/issuer": {
          "purposes": ["peer"],
          "organizations": ["did:web:peer.example"],
          "mechanism": "jwks",
          "jwks": `+jwksBlock+`
        }
      }
    }`)

	cfg, err := LoadTrustConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !cfg.For(PurposeLogin).IssuerTrusted("https://own.example/issuer") {
		t.Error("own issuer must be trusted for login")
	}
	if cfg.For(PurposeLogin).IssuerTrusted("https://peer.example/issuer") {
		t.Error("peer issuer must NOT be trusted for login")
	}
	if !cfg.For(PurposePeer).IssuerTrusted("https://peer.example/issuer") {
		t.Error("peer issuer must be trusted for peering")
	}
	// Mutual PoA binding: this instance verifies its own side too.
	if !cfg.For(PurposePeer).IssuerTrusted("https://own.example/issuer") {
		t.Error("own issuer must also be trusted for peering")
	}
	if _, err := cfg.For(PurposeLogin).IssuerJWKS("https://peer.example/issuer"); err == nil {
		t.Error("keys for an out-of-purpose issuer must not resolve")
	}
}

// An issuer may only speak for the organizations its entry names, so a trusted
// issuer cannot assert a party it was never entitled to represent.
func TestIssuerMayAttestOnlyListedOrganizations(t *testing.T) {
	path := writeTrust(t, `{
      "vcts": ["urn:dcs:poa:v1"],
      "issuers": {
        "https://peer.example/issuer": {
          "purposes": ["peer"],
          "organizations": ["did:web:peer.example"],
          "mechanism": "jwks",
          "jwks": `+jwksBlock+`
        }
      }
    }`)

	cfg, err := LoadTrustConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !cfg.IssuerMayAttest("https://peer.example/issuer", "did:web:peer.example") {
		t.Error("issuer must attest its own organization")
	}
	if cfg.IssuerMayAttest("https://peer.example/issuer", "did:web:own.example") {
		t.Error("issuer must NOT attest an organization it does not hold")
	}
	if cfg.IssuerMayAttest("https://peer.example/issuer", "") {
		t.Error("an empty organization must fail closed")
	}
	if cfg.IssuerMayAttest("https://unknown.example/issuer", "did:web:peer.example") {
		t.Error("an unknown issuer must attest nothing")
	}
}

// Configuration that cannot be enforced is refused at load, not at first use:
// a deployment learns on startup, not when a wallet arrives.
func TestLoadRefusesUnenforceableEntries(t *testing.T) {
	cases := map[string]string{
		"no purposes":                        `{"purposes":[],"organizations":["did:web:a.example"],"mechanism":"jwks","jwks":` + jwksBlock + `}`,
		"unknown purpose":                    `{"purposes":["admin"],"organizations":["did:web:a.example"],"mechanism":"jwks","jwks":` + jwksBlock + `}`,
		"no mechanism":                       `{"purposes":["login"],"organizations":["did:web:a.example"],"jwks":` + jwksBlock + `}`,
		"unsupported mechanism":              `{"purposes":["login"],"organizations":["did:web:a.example"],"mechanism":"did:ebsi"}`,
		"jwks without keys":                  `{"purposes":["login"],"organizations":["did:web:a.example"],"mechanism":"jwks"}`,
		"party issuer without organizations": `{"purposes":["login"],"organizations":[],"mechanism":"jwks","jwks":` + jwksBlock + `}`,
	}

	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTrust(t, `{"vcts":["urn:dcs:poa:v1"],"issuers":{"https://a.example/issuer":`+entry+`}}`)
			if _, err := LoadTrustConfig(path); err == nil {
				t.Fatalf("expected %s to be refused at load", name)
			}
		})
	}
}

// A pid issuer attests a person rather than a party, so it needs no
// organizations — but it still may not stand in for one.
func TestPIDIssuerNeedsNoOrganizations(t *testing.T) {
	path := writeTrust(t, `{
      "vcts": ["urn:eudi:pid:de:1"],
      "issuers": {
        "https://pid.example/issuer": {
          "purposes": ["pid"],
          "mechanism": "jwks",
          "jwks": `+jwksBlock+`
        }
      }
    }`)

	cfg, err := LoadTrustConfig(path)
	if err != nil {
		t.Fatalf("a pid issuer without organizations must load: %v", err)
	}
	if cfg.IssuerMayAttest("https://pid.example/issuer", "did:web:a.example") {
		t.Error("a pid issuer must not attest an organization")
	}
	if cfg.For(PurposeLogin).IssuerTrusted("https://pid.example/issuer") {
		t.Error("a pid issuer must not grant login")
	}
}

// The wildcard must be written out; it is not what an absent list means.
func TestOrganizationsWildcard(t *testing.T) {
	path := writeTrust(t, `{
      "vcts": ["urn:dcs:poa:v1"],
      "issuers": {
        "https://tenants.example/issuer": {
          "purposes": ["login"],
          "organizations": ["*"],
          "mechanism": "jwks",
          "jwks": `+jwksBlock+`
        }
      }
    }`)

	cfg, err := LoadTrustConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.IssuerMayAttest("https://tenants.example/issuer", "Acme Corp") {
		t.Error("a wildcard issuer must attest any organization it names")
	}
	if cfg.IssuerMayAttest("https://tenants.example/issuer", "") {
		t.Error("even a wildcard issuer must not attest an empty organization")
	}
}

func TestDevTrustConfigLoads(t *testing.T) {
	cfg, err := LoadTrustConfig("../../../config/oid4vp/trust.dev.json")
	if err != nil {
		t.Fatalf("shipped dev trust config must load: %v", err)
	}
	if len(cfg.Issuers) == 0 {
		t.Fatal("dev trust config has no issuers")
	}
	for iss, entry := range cfg.Issuers {
		if len(entry.Purposes) == 0 {
			t.Errorf("issuer %q has no purposes", iss)
		}
		var probe map[string]any
		if err := json.Unmarshal(entry.JWKS, &probe); err != nil {
			t.Errorf("issuer %q jwks is not an object: %v", iss, err)
		}
	}
}

type stubFetcher struct {
	docs map[string][]byte
	err  error
}

func (s stubFetcher) Fetch(url string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	body, ok := s.docs[url]
	if !ok {
		return nil, fmt.Errorf("no document at %s", url)
	}
	return body, nil
}

// did:web resolution must hit the identifier's own document, with the port
// decoded back out of the percent-encoded authority.
func TestDIDWebURLMapping(t *testing.T) {
	cases := map[string]string{
		"did:web:example.com":                    "https://example.com/.well-known/did.json",
		"did:web:example.com:issuer":             "https://example.com/issuer/did.json",
		"did:web:dcs-b.localhost%3A18080:issuer": "https://dcs-b.localhost:18080/issuer/did.json",
	}
	for iss, want := range cases {
		got, err := didWebURL(iss)
		if err != nil {
			t.Errorf("%s: %v", iss, err)
			continue
		}
		if got != want {
			t.Errorf("%s → %s, want %s", iss, got, want)
		}
	}
	if _, err := didWebURL("https://example.com/issuer"); err == nil {
		t.Error("a non did:web identifier must be refused")
	}
}

func TestResolveKeysByMechanism(t *testing.T) {
	didDoc := []byte(`{"verificationMethod":[{"publicKeyJwk":{"kty":"EC","crv":"P-256","x":"VlBNhqQn6gLyQXqKkLDHBwXlJsi0IES4OovRv9FrAHI","y":"vZMT1rkIeVaj7Om-FuIIcMHA1-xHtSk3OTGgovfeHCk"}}]}`)
	cfg := &TrustConfig{
		VCTs: []string{"urn:dcs:poa:v1"},
		Issuers: map[string]TrustedIssuer{
			"did:web:example.com:issuer":  {Purposes: []Purpose{PurposePeer}, Organizations: []string{"*"}, Mechanism: MechanismDIDWeb},
			"https://x5c.example/issuer":  {Purposes: []Purpose{PurposePID}, Mechanism: MechanismX5C},
			"https://orce.example/issuer": {Purposes: []Purpose{PurposePeer}, Organizations: []string{"*"}, Mechanism: MechanismORCE},
		},
	}
	cfg.SetKeyFetcher(stubFetcher{docs: map[string][]byte{
		"https://example.com/issuer/did.json": didDoc,
	}})

	keys, err := cfg.resolveIssuerKeys("did:web:example.com:issuer")
	if err != nil {
		t.Fatalf("did:web resolve: %v", err)
	}
	if !strings.Contains(string(keys), "VlBNhqQn6gLy") {
		t.Errorf("did:web keys not returned: %s", keys)
	}

	// An x5c issuer resolves to no JWKS: its key arrives in the chain.
	keys, err = cfg.resolveIssuerKeys("https://x5c.example/issuer")
	if err != nil || len(keys) != 0 {
		t.Errorf("x5c must resolve to no jwks, got %q err %v", keys, err)
	}

	// orce without a configured endpoint must say so, not fail obscurely.
	if _, err := cfg.resolveIssuerKeys("https://orce.example/issuer"); err == nil ||
		!strings.Contains(err.Error(), "no resolver endpoint is configured") {
		t.Errorf("expected a clear orce configuration error, got %v", err)
	}
}

// Federation cannot require editing every instance's trust file whenever a
// member is onboarded, so a peer's issuer is trusted dynamically — bounded by
// its own authority, and authorized separately by the ADR-19 gate and the PDP.
func TestDynamicPeerTrust(t *testing.T) {
	cfg := &TrustConfig{
		VCTs:        []string{"urn:dcs:poa:v1"},
		PeerDynamic: true,
		Issuers: map[string]TrustedIssuer{
			"https://own.example/issuer": {
				Purposes: []Purpose{PurposeLogin}, Organizations: []string{"did:web:own.example"},
				Mechanism: MechanismJWKS, JWKS: json.RawMessage(jwksBlock),
			},
		},
	}

	unlisted := "did:web:newpeer.example:issuer"

	if !cfg.For(PurposePeer).IssuerTrusted(unlisted) {
		t.Error("an unlisted did:web peer issuer must be verifiable for peering")
	}
	// Trusting it is worthless if no key can be resolved for it. Asserting only
	// the gate let this ship as dead code: IssuerTrusted said yes while
	// resolution rejected the same issuer, so no dynamic peer could ever be
	// verified.
	cfg.SetKeyFetcher(stubFetcher{docs: map[string][]byte{
		"https://newpeer.example/issuer/did.json": []byte(`{"verificationMethod":[{"publicKeyJwk":{"kty":"EC","crv":"P-256","x":"VlBNhqQn6gLyQXqKkLDHBwXlJsi0IES4OovRv9FrAHI","y":"vZMT1rkIeVaj7Om-FuIIcMHA1-xHtSk3OTGgovfeHCk"}}]}`),
	}})
	keys, err := cfg.For(PurposePeer).IssuerJWKS(unlisted)
	if err != nil || !strings.Contains(string(keys), "VlBNhqQn6gLy") {
		t.Fatalf("a dynamic peer's key must resolve from its own DID document: keys=%q err=%v", keys, err)
	}
	if usesX5C, err := cfg.For(PurposePeer).IssuerUsesX5C(unlisted); err != nil || usesX5C {
		t.Errorf("a dynamic peer publishes a DID document, not a chain: %v %v", usesX5C, err)
	}
	// Access to this deployment stays the operator's explicit decision.
	if cfg.For(PurposeLogin).IssuerTrusted(unlisted) {
		t.Error("a dynamic peer issuer must NOT grant login")
	}
	if cfg.For(PurposePID).IssuerTrusted(unlisted) {
		t.Error("a dynamic peer issuer must NOT serve as a PID issuer")
	}
	// It speaks for its own party and no other.
	if !cfg.IssuerMayAttest(unlisted, "did:web:newpeer.example") {
		t.Error("a peer issuer must attest its own authority")
	}
	if cfg.IssuerMayAttest(unlisted, "did:web:own.example") {
		t.Error("a peer issuer must not attest another party")
	}

	// Without the flag, nothing is trusted implicitly.
	cfg.PeerDynamic = false
	if cfg.For(PurposePeer).IssuerTrusted(unlisted) {
		t.Error("dynamic peer trust must be opt-in")
	}
}

func TestPeerAuthority(t *testing.T) {
	cases := map[string]string{
		"did:web:example.com:issuer":             "did:web:example.com",
		"did:web:example.com":                    "did:web:example.com",
		"did:web:dcs-b.localhost%3A18080:issuer": "did:web:dcs-b.localhost%3A18080",
		"https://example.com/issuer":             "",
	}
	for iss, want := range cases {
		if got := peerAuthority(iss); got != want {
			t.Errorf("%s → %q, want %q", iss, got, want)
		}
	}
}

// The configuration decides how an issuer's key is resolved. If the credential
// decided, anyone holding a certificate under any configured anchor could
// present it for an issuer that publishes a JWKS and be believed.
func TestMechanismIsAuthoritativeNotTheCredential(t *testing.T) {
	path := writeTrust(t, `{
      "vcts": ["urn:dcs:poa:v1"],
      "issuers": {
        "https://jwks.example/issuer": {
          "purposes": ["login"], "organizations": ["did:web:jwks.example"],
          "mechanism": "jwks", "jwks": `+jwksBlock+`
        },
        "https://chain.example/issuer": {
          "purposes": ["pid"], "mechanism": "x5c"
        }
      }
    }`)
	cfg, err := LoadTrustConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	usesX5C, err := cfg.For(PurposeLogin).IssuerUsesX5C("https://jwks.example/issuer")
	if err != nil || usesX5C {
		t.Errorf("a jwks issuer must not be resolvable through a chain: %v %v", usesX5C, err)
	}
	usesX5C, err = cfg.For(PurposePID).IssuerUsesX5C("https://chain.example/issuer")
	if err != nil || !usesX5C {
		t.Errorf("an x5c issuer must resolve through its chain: %v %v", usesX5C, err)
	}
	// Out of purpose, the question is not answerable at all.
	if _, err := cfg.For(PurposeLogin).IssuerUsesX5C("https://chain.example/issuer"); err == nil {
		t.Error("mechanism must not be resolvable for an issuer outside its purpose")
	}
}

// An explicit entry is the operator's complete answer: withholding a purpose
// denies it, rather than falling through to the dynamic peer path.
func TestExplicitEntryDeniesRatherThanFallingThrough(t *testing.T) {
	cfg := &TrustConfig{
		VCTs:        []string{"urn:dcs:poa:v1"},
		PeerDynamic: true,
		Issuers: map[string]TrustedIssuer{
			"did:web:listed.example:issuer": {
				Purposes: []Purpose{PurposeLogin}, Organizations: []string{"did:web:listed.example"},
				Mechanism: MechanismJWKS, JWKS: json.RawMessage(jwksBlock),
			},
		},
	}
	if cfg.For(PurposePeer).IssuerTrusted("did:web:listed.example:issuer") {
		t.Error("an entry granting only login must not also grant peer via the dynamic path")
	}
}
