package sdjwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func disclosureFor(t *testing.T, name string, value any) string {
	t.Helper()
	raw, err := json.Marshal([]any{"salt", name, value})
	if err != nil {
		t.Fatalf("marshal disclosure: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// The registered claims are checked against the raw signed payload, but `iss`,
// `cnf` and `status` are read again from the merged map — to pick the issuer
// entry whose organization entitlement is checked, to bind the holder, and to
// find the revocation list. A disclosure carrying one of those would move a
// check to a target the issuer never signed for.
func TestDisclosureCannotCarryARegisteredClaim(t *testing.T) {
	for _, name := range []string{"iss", "sub", "cnf", "status", "vct", "exp", "_sd", "_sd_alg"} {
		t.Run(name, func(t *testing.T) {
			_, err := MergeDisclosedClaims(
				jwt.MapClaims{"iss": "did:web:real.example"},
				[]string{disclosureFor(t, name, "attacker-chosen")},
			)
			if err == nil {
				t.Fatalf("a disclosure carrying %q was merged", name)
			}
			if !strings.Contains(err.Error(), "registered claim") {
				t.Fatalf("refused for the wrong reason: %v", err)
			}
		})
	}
}

func TestDisclosureCannotOverrideASignedClaim(t *testing.T) {
	_, err := MergeDisclosedClaims(
		jwt.MapClaims{"organization": "Acme Corp"},
		[]string{disclosureFor(t, "organization", "Someone Else")},
	)
	if err == nil {
		t.Fatal("a disclosure overrode a claim the issuer signed")
	}
	if !strings.Contains(err.Error(), "overrides a claim") {
		t.Fatalf("refused for the wrong reason: %v", err)
	}
}

// The ordinary case still works: a selectively disclosed claim the issuer left
// out of the payload merges in.
func TestDisclosedClaimStillMerges(t *testing.T) {
	claims, err := MergeDisclosedClaims(
		jwt.MapClaims{"iss": "did:web:real.example"},
		[]string{disclosureFor(t, "organization", "Acme Corp")},
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if claims["organization"] != "Acme Corp" {
		t.Fatalf("organization did not merge: %v", claims)
	}
}
