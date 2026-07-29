package dcstodcs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/auth/oid4vp"
	smdb "digital-contracting-service/internal/signingmanagement/db"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
)

// SignatoryPoA is the Power-of-Attorney evidence behind one applied signature,
// on the wire between two instances: the party the credential authorizes and
// the presentation the signatory's wallet delivered at the ceremony.
type SignatoryPoA struct {
	Party        string
	Presentation string
}

// SignatoryPoAs reads the evidence retained for the signatures an instance
// applied to a contract.
type SignatoryPoAs interface {
	ForContract(ctx context.Context, contractIRI string) ([]SignatoryPoA, error)
}

// CeremonyPoAs is the production SignatoryPoAs: the credential is retained on
// the signing ceremony that consumed it, next to the PID presentation.
type CeremonyPoAs struct {
	DB           *sqlx.DB
	CeremonyRepo smdb.CeremonyRepo
}

func (c *CeremonyPoAs) ForContract(ctx context.Context, contractIRI string) ([]SignatoryPoA, error) {
	tx, err := c.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	applied, err := c.CeremonyRepo.ListAppliedPoAs(ctx, tx, contractIRI)
	if err != nil {
		return nil, err
	}

	evidence := make([]SignatoryPoA, 0, len(applied))
	for _, poa := range applied {
		evidence = append(evidence, SignatoryPoA{Party: poa.FieldName, Presentation: poa.Presentation})
	}
	return evidence, nil
}

// WireSignatoryPoAs converts retained evidence to the ship payload.
func WireSignatoryPoAs(evidence []SignatoryPoA) []*dcstodcs.DCSToDCSSignatoryPoA {
	if len(evidence) == 0 {
		return nil
	}
	wire := make([]*dcstodcs.DCSToDCSSignatoryPoA, 0, len(evidence))
	for _, poa := range evidence {
		wire = append(wire, &dcstodcs.DCSToDCSSignatoryPoA{Party: poa.Party, Presentation: poa.Presentation})
	}
	return wire
}

// ReceivedSignatoryPoAs converts a ship payload back to evidence.
func ReceivedSignatoryPoAs(wire []*dcstodcs.DCSToDCSSignatoryPoA) []SignatoryPoA {
	if len(wire) == 0 {
		return nil
	}
	evidence := make([]SignatoryPoA, 0, len(wire))
	for _, poa := range wire {
		if poa == nil {
			continue
		}
		evidence = append(evidence, SignatoryPoA{Party: poa.Party, Presentation: poa.Presentation})
	}
	return evidence
}

// CounterpartyPoAGate verifies the Power-of-Attorney evidence a peer ships with
// a contract (ADR-31): the counterparty's side of the mutual binding.
//
// Evidence that is present and does not verify refuses the exchange, like any
// other trust-gate denial. Evidence that is ABSENT does not: a peer that
// retains none still federates, and a party that signed without a Power of
// Attorney is raised by the compliance viewer from the contract itself
// (signingmanagement/command/compliance.go), which is where that finding has
// always come from.
type CounterpartyPoAGate struct {
	Trust *oid4vp.TrustConfig
	// Verify is the credential check, defaulting to oid4vp.VerifyCounterpartyPoA.
	// Held as a field so the party-matching rules below can be exercised for
	// what they accept as well as what they refuse: with the real verifier they
	// are only reachable by minting a genuine credential, and the acceptance
	// path went untested long enough to ship a join that never matched.
	Verify func(presentation string, trust *oid4vp.TrustConfig, expected oid4vp.CounterpartyPoAExpectation) (*oid4vp.CounterpartyPoA, error)
}

func (g *CounterpartyPoAGate) verify() func(string, *oid4vp.TrustConfig, oid4vp.CounterpartyPoAExpectation) (*oid4vp.CounterpartyPoA, error) {
	if g.Verify != nil {
		return g.Verify
	}
	return oid4vp.VerifyCounterpartyPoA
}

// Check verifies each shipped credential against the contract the same ship
// carried. Nothing the peer asserts alongside the credential is believed: the
// party's signatory and its claimed Power of Attorney are read from the
// payload, which the content gate has already tied to the PDF's visible text.
func (g *CounterpartyPoAGate) Check(peerDID string, payload []byte, evidence []SignatoryPoA) error {
	if len(evidence) == 0 {
		return nil
	}

	deny := func(err error) error {
		return &GateError{Kind: PoAFailure, PeerDID: peerDID, Err: err}
	}

	if g.Trust == nil {
		return deny(fmt.Errorf("counterparty Power of Attorney: no issuer trust is configured, so nothing shipped can be verified"))
	}

	parties, err := signedPartiesOf(payload)
	if err != nil {
		return deny(fmt.Errorf("counterparty Power of Attorney: %w", err))
	}

	verified := make(map[string]bool, len(evidence))
	for _, poa := range evidence {
		// The credential authorizes an organization, and the contract records
		// which party that organization authorized. Joining on that recorded
		// authorization rather than on the party's own IRI is what makes this
		// work for both contract shapes: an auto-seeded signature field is named
		// for the signing instance's DID, so organization and party IRI coincide,
		// while an authored multi-signatory contract names its fields freely and
		// the two differ.
		organization := strings.TrimSpace(poa.Party)
		party, node, found := partyAuthorizedBy(parties, organization)
		if !found {
			return deny(fmt.Errorf(
				"counterparty Power of Attorney: the shipped contract records no signed party authorized by %q", organization))
		}
		// A peer ships the evidence behind the signatures IT applied. Evidence
		// for anyone else is a credential obtained in some other exchange being
		// replayed here: the presentation carries no audience or nonce this
		// instance could check, so without this it would verify on its own merits
		// and vouch for a party the shipper has nothing to do with.
		//
		// Only a did:web organization can be held against the peer's identity. An
		// authored contract may name its parties anything, and there the issuer's
		// entitlement to attest that organization is the only bound there is.
		if strings.HasPrefix(organization, "did:web:") && organization != strings.TrimSpace(peerDID) {
			return deny(fmt.Errorf("counterparty Power of Attorney: peer %q shipped evidence for %q, which is not its own",
				peerDID, organization))
		}
		_, err := g.verify()(poa.Presentation, g.Trust, oid4vp.CounterpartyPoAExpectation{
			Organization: organization,
			SignatoryDID: node.Signatory,
		})
		if err != nil {
			return deny(fmt.Errorf("counterparty Power of Attorney for party %q: %w", party, err))
		}
		verified[party] = true
	}

	// Shipping evidence for one signature and omitting it for another would let
	// a peer choose per signature which of them gets verified. Absence is
	// tolerated wholesale, not selectively.
	for party, node := range parties {
		if node.PoAOrganization == "" || verified[party] {
			continue
		}
		return deny(fmt.Errorf(
			"counterparty Power of Attorney: the shipped contract records party %q as authorized by %q, but the ship carries evidence only for %v",
			party, node.PoAOrganization, keysOf(verified)))
	}
	return nil
}

// partyAuthorizedBy finds the signed party the contract records as authorized by
// this organization.
func partyAuthorizedBy(parties map[string]signedParty, organization string) (string, signedParty, bool) {
	if organization == "" {
		return "", signedParty{}, false
	}
	for party, node := range parties {
		if node.PoAOrganization == organization {
			return party, node, true
		}
	}
	return "", signedParty{}, false
}

func keysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// signedParty is a party node of a shipped contract that carries a signature.
type signedParty struct {
	Signatory       string
	PoAOrganization string
}

// signedPartiesOf reads the contract's signed parties out of the machine-
// readable payload embedded in the shipped PDF, keyed by party IRI.
func signedPartiesOf(payload []byte) (map[string]signedParty, error) {
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("decode contract payload: %w", err)
	}
	nodes, _ := doc["dcs:parties"].([]any)
	parties := make(map[string]signedParty, len(nodes))
	for _, rawNode := range nodes {
		node, ok := rawNode.(map[string]any)
		if !ok {
			continue
		}
		signatory := jsonLDIRI(node["dcs:hasSignatory"])
		if signatory == "" {
			continue
		}
		id, _ := node["@id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parties[id] = signedParty{
			Signatory:       signatory,
			PoAOrganization: jsonLDIRI(node["dcs:hasPowerOfAttorney"]),
		}
	}
	return parties, nil
}

// jsonLDIRI reads an IRI from a value that is either {"@id": iri} or a bare
// string.
func jsonLDIRI(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		id, _ := typed["@id"].(string)
		return strings.TrimSpace(id)
	case string:
		return strings.TrimSpace(typed)
	}
	return ""
}
