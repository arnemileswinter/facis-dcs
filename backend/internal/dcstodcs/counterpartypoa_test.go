package dcstodcs

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/auth/oid4vp"
)

const (
	testSignedParty     = "did:web:peer.example"
	testSignedSignatory = "did:jwk:eyJrdHkiOiJFQyJ9"
)

// summaryVC is a ContractSigningSummaryCredential as the shipping instance
// embeds it (DCS-FR-SM-08): field_name is the party the signature was made for,
// credentialSubject.id the signatory that made it.
func summaryVC(organization, signatory string) string {
	return `{
	  "@context": ["https://www.w3.org/ns/credentials/v2"],
	  "type": ["VerifiableCredential", "ContractSigningSummaryCredential"],
	  "issuer": "did:web:peer.example",
	  "credentialSubject": {
	    "id": "` + signatory + `",
	    "field_name": "` + organization + `",
	    "contract_id": "urn:contract:1"
	  }
	}`
}

// shippedEvidence is the signing evidence embedded in a PDF shipped by a peer.
func shippedEvidence(organizations ...string) ShippedSignatures {
	vcs := make([]string, 0, len(organizations))
	for _, org := range organizations {
		signatory := testSignedSignatory
		if org != testSignedParty {
			signatory = "did:jwk:" + org
		}
		vcs = append(vcs, summaryVC(org, signatory))
	}
	return ShippedSignatures{Evidence: []byte("[" + strings.Join(vcs, ",") + "]")}
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
	require.NoError(t, gate.Check(testSignedParty, shippedEvidence(testSignedParty), nil))
}

// Present evidence that cannot be verified refuses the exchange rather than
// being ignored — including when this instance has no trust configuration to
// verify it against.
func TestCounterpartyPoAGate_EvidenceWithoutTrustConfigIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: nil}
	err := gate.Check(testSignedParty, shippedEvidence(testSignedParty), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "no issuer trust is configured")
}

// Evidence is matched against the contract the same ship carried, so a
// credential for a party that did not sign it authorizes nothing here.
func TestCounterpartyPoAGate_EvidenceForAnUnsignedPartyIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, shippedEvidence(testSignedParty), []SignatoryPoA{
		{Party: "did:web:the-other-party.example", Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "records no signed party authorized by")
}

// The contract's own dcs:hasPowerOfAttorney claim has to agree with the
// credential being offered for it; a peer cannot ship evidence for one
// authorization while the contract records another.
func TestCounterpartyPoAGate_ContractRecordsADifferentAuthorizationIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, shippedEvidence("did:web:impostor.example"), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "authorized by")
}

func TestCounterpartyPoAGate_UnreadableSigningEvidenceIsRefused(t *testing.T) {
	gate := CounterpartyPoAGate{Trust: &oid4vp.TrustConfig{}}
	err := gate.Check(testSignedParty, ShippedSignatures{Evidence: []byte("not json")}, []SignatoryPoA{
		{Party: testSignedParty, Presentation: "irrelevant"},
	})
	assert.Contains(t, gateError(t, err).Error(), "decode signing evidence")
}

func TestSignedPartiesOf_ReadsOnlySignedParties(t *testing.T) {
	parties, err := signedPartiesOf(shippedEvidence(testSignedParty))
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

	err := gate.Check(testSignedParty, shippedEvidence(testSignedParty), []SignatoryPoA{
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

	err := gate.Check("did:web:someone-else.example", shippedEvidence(testSignedParty), []SignatoryPoA{
		{Party: testSignedParty, Presentation: "a-genuine-presentation"},
	})

	require.Error(t, err)
	var gateErr *GateError
	require.True(t, errors.As(err, &gateErr))
	assert.Contains(t, gateErr.Error(), "which is not its own")
	assert.Empty(t, seen, "a credential for another party must be refused before it is verified")
}

// The return leg of a two-instance signing: A signs and ships, B signs on top
// and ships the double-signed contract back. It records BOTH parties as
// authorized, but B holds only its own presentation — the receive path verifies
// inbound evidence without retaining it — so B can only ever ship one.
//
// Requiring evidence for every authorized party made this ship impossible while
// leaving a peer that wants nothing verified free to send an empty list, so the
// requirement only ever refused honest peers.
func TestCounterpartyPoAGate_DoubleSignedReturnLegIsAccepted(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	doubleSigned := shippedEvidence("did:web:a.example", testSignedParty)

	require.NoError(t, gate.Check(testSignedParty, doubleSigned, []SignatoryPoA{
		{Party: testSignedParty, Presentation: "b-genuine-presentation"},
	}))

	require.Len(t, seen, 1, "the shipper's own evidence is still verified")
	assert.Equal(t, testSignedParty, seen[0].Organization)
	assert.Equal(t, testSignedSignatory, seen[0].SignatoryDID)
}

// An authored multi-signatory contract names its signature fields freely, so
// the organization a credential authorizes is not the party's own IRI. The join
// follows the authorization the contract records rather than the IRI.
func TestCounterpartyPoAGate_AuthoredFieldNamesStillJoin(t *testing.T) {
	var seen []oid4vp.CounterpartyPoAExpectation
	gate := acceptingGate(&seen)

	payload := shippedEvidence("Acme Corp")

	require.NoError(t, gate.Check(testSignedParty, payload, []SignatoryPoA{
		{Party: "Acme Corp", Presentation: "a-genuine-presentation"},
	}))
	require.Len(t, seen, 1)
	assert.Equal(t, "Acme Corp", seen[0].Organization)
}
