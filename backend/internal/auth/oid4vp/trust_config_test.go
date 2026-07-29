package oid4vp

import (
	"encoding/json"
	"os"
	"path/filepath"
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
