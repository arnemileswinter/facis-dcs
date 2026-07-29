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
	assert.Contains(t, gateError(t, err).Error(), "records no signed party authorized by")
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

// acceptingGate stands in for the credential check so the party-matching rules
// can be exercised for what they ACCEPT. It records what the gate asked to be
// verified, which is the part that has to line up with the contract.
func acceptingGate(seen *[]oid4vp.CounterpartyPoAExpectation) CounterpartyPoAGate {
	return CounterpartyPoAGate{
		Trust: &oid4vp.TrustConfig{},
		Verify: func(_ string, _ *oid4vp.TrustConfig, expected oid4vp.CounterpartyPoAExpectation) (*oid4vp.CounterpartyPoA, error) {
			*seen = append(*seen, expected)
			return &oid4vp.CounterpartyPoA{Organization: expected.Organization, SignatoryDID: expected.SignatoryDID}, nil
		},
	}
}

// The join has to match the ordinary two-instance ship, where the signing
// instance's DID is both the organization the credential authorizes and the
// party IRI the contract carries. Nothing asserted this before, and a join that
// could never match shipped as a result.
func TestCounterpartyPoAGate_VerifiedEvidenceIsAccepted(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	err := gate.Check(testSignedParty, signedContractPayload(testSignedParty), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "a-genuine-presentation"},
	})
	require.NoError(t, err)

	require.Len(t, seen, 1, "the shipped credential must actually be verified, not skipped")
	assert.Equal(t, testSignedParty, seen[0].Organization)
	assert.Equal(t, testSignedSignatory, seen[0].SignatoryDID,
		"the credential is bound to the signatory the shipped contract records, not to one the peer names")
}

// A peer ships the evidence behind its OWN signatures. A credential for another
// party is one obtained in a different exchange: it would verify on its own
// merits, because nothing in the presentation names this contract.
func TestCounterpartyPoAGate_EvidenceForAnotherPartyIsRefused(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	err := gate.Check("did:web:someone-else.example", signedContractPayload(testSignedParty), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "a-genuine-presentation"},
	})

	require.Error(t, err)
	var gateErr *GateError
	require.True(t, errors.As(err, &gateErr))
	assert.Contains(t, gateErr.Error(), "which is not its own")
	assert.Empty(t, seen, "a credential for another party must be refused before it is verified")
}

// Shipping evidence for one signature and omitting it for another would let a
// peer choose per signature which of them is checked.
func TestCounterpartyPoAGate_PartialEvidenceIsRefused(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	payload := []byte(`{
	  "dcs:parties": [
	    {"@id": "` + testSignedParty + `",
	     "dcs:hasSignatory": {"@id": "` + testSignedSignatory + `"},
	     "dcs:hasPowerOfAttorney": {"@id": "` + testSignedParty + `"}},
	    {"@id": "did:web:second.example",
	     "dcs:hasSignatory": {"@id": "did:jwk:second"},
	     "dcs:hasPowerOfAttorney": {"@id": "did:web:second.example"}}
	  ]
	}`)

	err := gate.Check(testSignedParty, payload, []SignatoryPoA{
		{Party: testSignedParty, Presentation: "a-genuine-presentation"},
	})

	require.Error(t, err)
	assert.Contains(t, gateError(t, err).Error(), "carries evidence only for")
}

// An authored multi-signatory contract names its signature fields freely, so
// the organization a credential authorizes is not the party's own IRI. The join
// follows the authorization the contract records rather than the IRI.
func TestCounterpartyPoAGate_AuthoredFieldNamesStillJoin(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	payload := []byte(`{
	  "dcs:parties": [
	    {"@id": "urn:contract:1#party-assignee",
	     "dcs:hasSignatory": {"@id": "` + testSignedSignatory + `"},
	     "dcs:hasPowerOfAttorney": {"@id": "Acme Corp"}}
	  ]
	}`)

	require.NoError(t, gate.Check(testSignedParty, payload, []SignatoryPoA{
		{Party: "Acme Corp", Presentation: "a-genuine-presentation"},
	}))
	require.Len(t, seen, 1)
	assert.Equal(t, "Acme Corp", seen[0].Organization)
}
