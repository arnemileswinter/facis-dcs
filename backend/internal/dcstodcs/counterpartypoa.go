package dcstodcs

import (
	"context"
	"encoding/json"
	"fmt"
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

	for _, poa := range evidence {
		party := strings.TrimSpace(poa.Party)
		node, signed := parties[party]
		if !signed {
			return deny(fmt.Errorf("counterparty Power of Attorney: party %q is not a signed party of the shipped contract", party))
		}
		if node.PoAOrganization != party {
			return deny(fmt.Errorf("counterparty Power of Attorney: the shipped contract records party %q as authorized by %q",
				party, node.PoAOrganization))
		}
		_, err := oid4vp.VerifyCounterpartyPoA(poa.Presentation, g.Trust, oid4vp.CounterpartyPoAExpectation{
			Organization: party,
			SignatoryDID: node.Signatory,
		})
		if err != nil {
			return deny(fmt.Errorf("counterparty Power of Attorney for party %q: %w", party, err))
		}
	}
	return nil
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
