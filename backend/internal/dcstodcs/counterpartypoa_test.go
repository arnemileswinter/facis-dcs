package dcstodcs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/auth/oid4vp"
)

const (
	testSignedParty     = "did:web:peer.example"
	testSignedSignatory = "did:jwk:eyJrdHkiOiJFQyJ9"
)

func signedContractPayload(poaOrganization string) []byte {
	return []byte(`{
	  "@type": "dcs:Contract",
	  "dcs:parties": [
	    {"@id": "` + testSignedParty + `",
	     "dcs:hasSignatory": {"@id": "` + testSignedSignatory + `"},
	     "dcs:hasPowerOfAttorney": {"@id": "` + poaOrganization + `"}},
	    {"@id": "did:web:the-other-party.example"}
	  ]
	}`)
}

func gateError(t *testing.T, err error) *GateError {
	t.Helper()
	require.Error(t, err)
	var gateErr *GateError
	require.True(t, errors.As(err, &gateErr), "a refusal must arrive as a GateError so it is recorded like any other trust denial")
	assert.Equal(t, PoAFailure, gateErr.Kind)
	assert.Equal(t, "did:web:peer.example", gateErr.PeerDID)
	return gateErr
}

// A peer that retains no Power-of-Attorney evidence still federates: absence is
// left to the compliance viewer, which reports a party that signed without one
// from the contract itself.
func TestCounterpartyPoAGate_AbsentEvidenceIsAccepted(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: nil}
	require.NoError(t, gate.Check(testSignedParty, signedContractPayload(testSignedParty), nil))
}

// Present evidence that cannot be verified refuses the exchange rather than
// being ignored — including when this instance has no trust configuration to
// verify it against.
func TestCounterpartyPoAGate_EvidenceWithoutTrustConfigIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: nil}
	err := gate.Check(testSignedParty, signedContractPayload(testSignedParty), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "no issuer trust is configured")
}

// Evidence is matched against the contract the same ship carried, so a
// credential for a party that did not sign it authorizes nothing here.
func TestCounterpartyPoAGate_EvidenceForAnUnsignedPartyIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, signedContractPayload(testSignedParty), []SignatoryPoA{
		{Party: "did:web:the-other-party.example", Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "is not a signed party")
}

// The contract's own dcs:hasPowerOfAttorney claim has to agree with the
// credential being offered for it; a peer cannot ship evidence for one
// authorization while the contract records another.
func TestCounterpartyPoAGate_ContractRecordsADifferentAuthorizationIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, signedContractPayload("did:web:impostor.example"), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "authorized by")
}

func TestCounterpartyPoAGate_UnparseableContractPayloadIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, []byte("not json"), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "decode contract payload")
}

func TestSignedPartiesOf_ReadsOnlySignedParties(t *testing.T) {
	parties, err := signedPartiesOf(signedContractPayload(testSignedParty))
	require.NoError(t, err)
	require.Len(t, parties, 1)
	assert.Equal(t, testSignedSignatory, parties[testSignedParty].Signatory)
	assert.Equal(t, testSignedParty, parties[testSignedParty].PoAOrganization)
}
