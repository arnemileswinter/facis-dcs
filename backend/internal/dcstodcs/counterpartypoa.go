package dcstodcs

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/auth/oid4vp"
	"digital-contracting-service/internal/base/identity"
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

// ShippedSignatures is what the peer's own signing evidence says about the
// signatures on the contract it shipped: the ContractSigningSummaryCredential(s)
// embedded in the PDF (DCS-FR-SM-08), and the means to verify them against the
// peer's key.
type ShippedSignatures struct {
	Evidence []byte
	VerifyVC func(vc json.RawMessage) error
}

// Check verifies each shipped Power of Attorney against the peer's own signing
// evidence.
//
// The attribution is read from the signing summary VC, not from dcs:parties in
// the contract payload. The payload embedded in a signed PDF is pinned before
// the wallet signs it, so the signatory and the authority — recorded when the
// signature is applied — are not in it and cannot be: writing them there would
// change the bytes the signature covers. DCS-FR-SM-08 already requires the
// summary as a VC embedded in the PDF/A-3, which is issuer-signed by the
// shipping instance rather than being a bare assertion beside the credential.
func (g *CounterpartyPoAGate) Check(peerDID string, signed ShippedSignatures, evidence []SignatoryPoA) error {
	if len(evidence) == 0 {
		return nil
	}

	deny := func(err error) error {
		return &GateError{Kind: PoAFailure, PeerDID: peerDID, Err: err}
	}

	if g.Trust == nil {
		return deny(fmt.Errorf("counterparty Power of Attorney: no issuer trust is configured, so nothing shipped can be verified"))
	}

	parties, err := signedPartiesOf(signed)
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
		party, node, err := partyAuthorizedBy(parties, organization)
		if err != nil {
			return deny(fmt.Errorf("counterparty Power of Attorney: %w", err))
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
		if strings.HasPrefix(organization, "did:web:") && !identity.SameDIDWeb(organization, strings.TrimSpace(peerDID)) {
			return deny(fmt.Errorf("counterparty Power of Attorney: peer %q shipped evidence for %q, which is not its own",
				peerDID, organization))
		}
		if _, err := g.verify()(poa.Presentation, g.Trust, oid4vp.CounterpartyPoAExpectation{
			Organization: organization,
			SignatoryDID: node.Signatory,
		}); err != nil {
			return deny(fmt.Errorf("counterparty Power of Attorney for party %q: %w", party, err))
		}
		verified[party] = true
	}

	// Deliberately NOT required: evidence for every party the contract records as
	// authorized. A contract signed on both instances carries two such parties,
	// each authorized by a different peer, and neither peer holds the other's
	// presentation — the receive path verifies inbound evidence without retaining
	// it. Demanding all of it makes the return leg of every two-instance signing
	// unshippable, while a peer that wants nothing checked still just sends an
	// empty list. It would only ever fire against an honest peer.
	return nil
}

// partyAuthorizedBy finds the signed party the contract records as authorized by
// this organization.
//
// Two parties authorized by the same organization make the credential ambiguous
// — each records its own signatory, and the holder binding would be checked
// against whichever the map happened to yield first. Refusing is the only answer
// that is the same on every run.
func partyAuthorizedBy(parties map[string]signedParty, organization string) (string, signedParty, error) {
	if organization == "" {
		return "", signedParty{}, fmt.Errorf("evidence names no organization")
	}
	matches := make([]string, 0, 1)
	for party, node := range parties {
		if node.PoAOrganization == organization {
			matches = append(matches, party)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", signedParty{}, fmt.Errorf("the shipped contract records no signed party authorized by %q", organization)
	case 1:
		return matches[0], parties[matches[0]], nil
	default:
		return "", signedParty{}, fmt.Errorf(
			"the shipped contract records %v as all authorized by %q, so the credential does not identify which signature it stands behind",
			matches, organization)
	}
}

// signedParty is a party node of a shipped contract that carries a signature.
type signedParty struct {
	Signatory       string
	PoAOrganization string
}

// signedPartiesOf reads what the peer's signing evidence attests, keyed by the
// organization each signature was made for.
//
// A ceremony refuses unless the Power of Attorney authorizes exactly the party
// the signature field names (signingmanagement/command/ceremony.go), so the
// summary's field_name IS the organization its Power of Attorney must authorize,
// and credentialSubject.id is the signatory it must be held by.
func signedPartiesOf(signed ShippedSignatures) (map[string]signedParty, error) {
	if len(signed.Evidence) == 0 {
		return nil, fmt.Errorf("the shipped PDF carries no signing evidence, so nothing attests which signature this credential stands behind")
	}

	var summaries []json.RawMessage
	if err := json.Unmarshal(signed.Evidence, &summaries); err != nil {
		// A single-signature contract embeds one credential rather than a bundle.
		summaries = []json.RawMessage{signed.Evidence}
	}

	parties := make(map[string]signedParty, len(summaries))
	for _, raw := range summaries {
		var vc struct {
			Type              []string `json:"type"`
			CredentialSubject struct {
				ID        string `json:"id"`
				FieldName string `json:"field_name"`
			} `json:"credentialSubject"`
		}
		if err := json.Unmarshal(raw, &vc); err != nil {
			return nil, fmt.Errorf("decode signing evidence: %w", err)
		}
		if !slices.Contains(vc.Type, "ContractSigningSummaryCredential") {
			continue
		}
		organization := strings.TrimSpace(vc.CredentialSubject.FieldName)
		signatory := strings.TrimSpace(vc.CredentialSubject.ID)
		if organization == "" || signatory == "" {
			continue
		}
		// Verified against the peer's own key before anything it says is used:
		// unverified, this is the peer telling us who signed, which is what the
		// contract payload already was.
		if signed.VerifyVC != nil {
			if err := signed.VerifyVC(raw); err != nil {
				return nil, fmt.Errorf("signing evidence for %q does not verify against the peer's key: %w", organization, err)
			}
		}
		parties[organization] = signedParty{Signatory: signatory, PoAOrganization: organization}
	}
	return parties, nil
}
