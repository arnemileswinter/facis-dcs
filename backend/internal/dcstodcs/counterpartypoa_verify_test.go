package dcstodcs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/auth/oid4vp"
)

func signedEvidence(t *testing.T, proofMethod, proofPurpose string) ShippedSignatures {
	t.Helper()
	vc := `{
	  "type": ["VerifiableCredential", "ContractSigningSummaryCredential"],
	  "credentialSubject": {"id": "` + testSignedSignatory + `", "field_name": "` + testSignedParty + `"},
	  "proof": {"type": "DataIntegrityProof", "verificationMethod": "` + proofMethod + `", "proofPurpose": "` + proofPurpose + `"}
	}`
	return ShippedSignatures{
		Evidence:           []byte("[" + vc + "]"),
		VerificationMethod: testSignedParty + "#dcs-vc",
		VerifyVC:           func(json.RawMessage) error { return nil },
	}
}

// The one security control this gate rests on had no test at all: every case
// ran the branch where no verifier is configured.
func TestSigningEvidenceMustBeVerifiable(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, ShippedSignatures{Evidence: []byte("[]")}, []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	require.Error(t, err, "evidence with no means to verify it must be refused, not believed")
	assert.Contains(t, gateError(t, err).Error(), "no means to verify")
}

// A proof made with some other key the peer publishes must not be checked
// against the one we resolved for credential signing.
func TestSigningEvidenceMustNameThePeersCredentialKey(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	err := gate.Check(testSignedParty, signedEvidence(t, testSignedParty+"#dev-key-1", "assertionMethod"),
		[]SignatoryPoA{{Party: testSignedParty, Presentation: "p"}})

	require.Error(t, err)
	assert.Contains(t, gateError(t, err).Error(), "names verification method")
	assert.Empty(t, seen)
}

// A credential is an assertion; a proof made for another purpose does not
// establish one.
func TestSigningEvidenceMustProveAnAssertion(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	err := gate.Check(testSignedParty, signedEvidence(t, testSignedParty+"#dcs-vc", "authentication"),
		[]SignatoryPoA{{Party: testSignedParty, Presentation: "p"}})

	require.Error(t, err)
	assert.Contains(t, gateError(t, err).Error(), "not assertionMethod")
	assert.Empty(t, seen)
}

// A summary whose proof does not verify must stop the exchange before any of
// its claims are used.
func TestUnverifiableSigningEvidenceIsRefusedBeforeItsClaimsAreUsed(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	shipped := signedEvidence(t, testSignedParty+"#dcs-vc", "assertionMethod")
	shipped.VerifyVC = func(json.RawMessage) error { return assertErr("bad signature") }

	err := gate.Check(testSignedParty, shipped, []SignatoryPoA{{Party: testSignedParty, Presentation: "p"}})

	require.Error(t, err)
	assert.True(t, strings.Contains(gateError(t, err).Error(), "does not verify"))
	assert.Empty(t, seen, "an unverified summary must never reach the Power-of-Attorney check")
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// The ordinary case still passes once the proof names the right key and purpose.
func TestVerifiedSigningEvidenceIsAccepted(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)
	gate.Verify = func(_ string, _ *oid4vp.TrustConfig, e oid4vp.CounterpartyPoAExpectation) (*oid4vp.CounterpartyPoA, error) {
		seen = append(seen, e)
		return &oid4vp.CounterpartyPoA{}, nil
	}

	require.NoError(t, gate.Check(testSignedParty, signedEvidence(t, testSignedParty+"#dcs-vc", "assertionMethod"),
		[]SignatoryPoA{{Party: testSignedParty, Presentation: "p"}}))
	require.Len(t, seen, 1)
	assert.Equal(t, testSignedSignatory, seen[0].SignatoryDID)
}
