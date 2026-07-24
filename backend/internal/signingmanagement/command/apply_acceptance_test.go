package command

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCredentialTypeAtLeast locks the SES < AES < QES ranking the prepare-time
// fail-fast check and the submit-time level gate both rely on (SM-01, ADR-20).
func TestCredentialTypeAtLeast(t *testing.T) {
	require.True(t, credentialTypeAtLeast("QES", "AES"))
	require.True(t, credentialTypeAtLeast("QES", "QES"))
	require.True(t, credentialTypeAtLeast("AES", "AES"))
	require.False(t, credentialTypeAtLeast("AES", "QES"))
	require.False(t, credentialTypeAtLeast("", "AES"))
	require.True(t, credentialTypeAtLeast("aes", "AES"), "comparison is case-insensitive")
}

func TestPidGivenFamilyName(t *testing.T) {
	given, family := pidGivenFamilyName([]byte(`{"given_name":"Jane","family_name":"Doe"}`))
	require.Equal(t, "Jane", given)
	require.Equal(t, "Doe", family)

	given, family = pidGivenFamilyName([]byte(`{"givenName":"Jane","familyName":"Doe"}`))
	require.Equal(t, "Jane", given)
	require.Equal(t, "Doe", family)

	given, family = pidGivenFamilyName(nil)
	require.Equal(t, "", given)
	require.Equal(t, "", family)

	given, family = pidGivenFamilyName([]byte(`not json`))
	require.Equal(t, "", given)
	require.Equal(t, "", family)
}

// TestNamesMatch proves the sole-control name gate (ADR-20 item 4): the PID
// and the certificate must agree on both given name and surname, tolerant of
// case and whitespace, and an absent certificate name is never a match by
// omission — a certificate that carries no name at all must never pass.
func TestNamesMatch(t *testing.T) {
	require.True(t, namesMatch("Jane", "Doe", "jane", "  DOE  "), "case/whitespace tolerant")
	require.False(t, namesMatch("Jane", "Doe", "Jane", "Smith"), "surname mismatch")
	require.False(t, namesMatch("Jane", "Doe", "John", "Doe"), "given name mismatch")
	require.False(t, namesMatch("Jane", "Doe", "", ""), "no certificate name is never a match")
	require.False(t, namesMatch("", "", "Jane", "Doe"), "no PID name is never a match")
}

// TestJWSPayloadAndHeaderRoundTrip proves the nonce-binding and payload-pin
// extraction helpers correctly recover a custom protected-header claim and
// the raw payload from a compact JWS, without needing a real signature — the
// signature itself is DSS's job; these just decode what DSS already
// validated (ADR-20 item 1/2).
func TestJWSPayloadAndHeaderRoundTrip(t *testing.T) {
	header := map[string]any{"alg": "ES256", "nonce": "abc-123"}
	headerBytes, err := json.Marshal(header)
	require.NoError(t, err)
	payload := []byte(`{"dcs:contractDid":"did:web:example#1"}`)

	compact := base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString([]byte("sig"))

	nonce, err := jwsProtectedHeaderClaim(compact, "nonce")
	require.NoError(t, err)
	require.Equal(t, "abc-123", nonce)

	missing, err := jwsProtectedHeaderClaim(compact, "not_present")
	require.NoError(t, err)
	require.Equal(t, "", missing)

	got, err := jwsPayloadBytes(compact)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	_, err = jwsPayloadBytes("not-a-jws")
	require.Error(t, err)
	_, err = jwsProtectedHeaderClaim("not-a-jws", "nonce")
	require.Error(t, err)
}
