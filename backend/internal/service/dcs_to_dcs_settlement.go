package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goa.design/clue/log"

	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/base/jades"
	trustgate "digital-contracting-service/internal/dcstodcs"
	db2 "digital-contracting-service/internal/dcstodcs/db"

	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
)

// settlementClockSkew is how far into the future a peer's claimed settlement
// time may sit before the artifact is refused: enough for two instances whose
// clocks are merely not synchronized, not enough to pre-date a settlement of a
// document that does not exist yet. A settlement never expires — it is voided
// by the document changing, not by time.
const settlementClockSkew = 5 * time.Minute

// PostSettlement receives the counterparty's evidence that it reached its own
// settled state on a named version of this contract. Signing claims both
// parties agreed the same document, and ADR-13 keeps intrinsic state local, so
// the peer's agreement is knowable here only as an artifact it signed and
// shipped — held locally, re-verifiable, and bound to the document this
// instance itself holds. Every refusal names the check that failed.
func (s *dcsToDcssrvc) PostSettlement(ctx context.Context, req *dcstodcs.DCSToDCSContractSettlementRequest) (res *dcstodcs.DCSToDCSContractSettlementResponse, err error) {
	remoteDIDDocument, err := fetchPeerDIDDocument(req.FromPeerDid)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if err := remoteDIDDocument.VerifyPeerChallenge(s.TrustPool, []byte(req.SecretValue), req.SecretHash); err != nil {
		return nil, contractworkflowengine.MakeBadRequest(
			fmt.Errorf("post_settlement rejected: peer %s did not authenticate: %w", req.FromPeerDid, err))
	}

	localPeer, err := s.DIDDocument.GetID()
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if identity.SameDIDWeb(req.FromPeerDid, localPeer) {
		return nil, contractworkflowengine.MakeBadRequest(
			errors.New("post_settlement rejected: a settlement shipped by this instance to itself is no counterparty evidence"))
	}

	// Federation trust gate (ADR-19), exactly as the PDF ship applies it: a
	// peer this instance may not federate with cannot deposit evidence here.
	if err := s.TrustGate.Check(ctx, req.FromPeerDid, trustgate.Inbound, req.ContractIri, ""); err != nil {
		var gateErr *trustgate.GateError
		if errors.As(err, &gateErr) {
			if incidentErr := trustgate.RecordDenialIncident(ctx, s.DB, req.ContractIri, trustgate.Inbound, gateErr); incidentErr != nil {
				log.Printf(ctx, "could not record trust gate denial incident for %s: %v", req.ContractIri, incidentErr)
			}
		}
		return nil, contractworkflowengine.MakeBadRequest(
			fmt.Errorf("post_settlement rejected: peer %s does not pass the federation trust gate: %w", req.FromPeerDid, err))
	}

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	contract, err := s.CRepo.ReadDataByDID(ctx, tx, req.ContractIri)
	if err != nil {
		return nil, contractworkflowengine.MakeBadRequest(
			fmt.Errorf("post_settlement rejected: this instance holds no copy of contract %s: %w", req.ContractIri, err))
	}
	contractDocument := []byte(`{}`)
	if contract.ContractData != nil && contract.ContractData.IsNotNullValue() {
		contractDocument = []byte(*contract.ContractData)
	}
	documentDigest, err := jades.ContractDocumentDigest(contractDocument)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	var parties []string
	if contract.Responsible != nil {
		parties = contract.Responsible.GetParties()
	}
	previous, err := s.SRepo.GetSettlement(ctx, tx, req.ContractIri, req.FromPeerDid)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	settlement, err := verifyShippedSettlement(req.SettlementJades, req.FromPeerDid, remoteDIDDocument, localSettlementContext{
		ContractIRI:    req.ContractIri,
		LocalPeer:      localPeer,
		DocumentDigest: documentDigest,
		Parties:        parties,
		Previous:       previous,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		return nil, contractworkflowengine.MakeBadRequest(fmt.Errorf("post_settlement rejected: %w", err))
	}

	if err := s.SRepo.UpsertSettlement(ctx, tx, *settlement); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	return &dcstodcs.DCSToDCSContractSettlementResponse{FromPeerDid: localPeer}, nil
}

// localSettlementContext is what this instance itself knows about the contract
// a peer claims to have settled — the ground truth every claim is checked
// against, never the shipped artifact's own account of it.
type localSettlementContext struct {
	ContractIRI    string
	LocalPeer      string
	DocumentDigest string
	Parties        []string
	Previous       *db2.Settlement
	Now            time.Time
}

// verifyShippedSettlement verifies a peer's settlement artifact and returns
// the record to store. It refuses — naming the failing check — a JAdES that
// does not verify, one signed by a key the peer does not publish for
// assertions, an artifact that is not a settlement, that names another
// contract, another audience or a non-party as settler, that does not
// re-canonicalize to the bytes that were signed, that settles a document other
// than the one this instance holds, or that is timestamped implausibly far
// ahead or behind a settlement already held from that peer.
func verifyShippedSettlement(jadesSignature, fromPeerDID string, remoteDIDDocument *identity.DIDDocument, local localSettlementContext) (*db2.Settlement, error) {
	payload, leafKey, err := jades.Verify(jadesSignature)
	if err != nil {
		return nil, fmt.Errorf("settlement JAdES does not verify: %w", err)
	}
	if !remoteDIDDocument.PublishesKeyFor(identity.PurposeAssertion, leafKey) {
		return nil, fmt.Errorf("settlement JAdES x5c leaf key is not published by peer %s as an %s key",
			fromPeerDID, identity.PurposeAssertion)
	}

	var claimed struct {
		Type            string `json:"@type"`
		ContractDID     string `json:"dcs:contractDid"`
		ContractVersion int    `json:"dcs:contractVersion"`
		DocumentDigest  string `json:"dcs:contractDocumentDigest"`
		SettledBy       string `json:"dcs:settledBy"`
		SettledWith     string `json:"dcs:settledWith"`
		SettledAt       string `json:"dcs:settledAt"`
	}
	if err := json.Unmarshal(payload, &claimed); err != nil {
		return nil, fmt.Errorf("could not decode the settlement payload: %w", err)
	}
	if claimed.Type != jades.SettlementType {
		return nil, fmt.Errorf("signed payload is a %q, not a %s", claimed.Type, jades.SettlementType)
	}
	for _, required := range []struct{ field, value string }{
		{"dcs:contractDid", claimed.ContractDID},
		{"dcs:contractDocumentDigest", claimed.DocumentDigest},
		{"dcs:settledBy", claimed.SettledBy},
		{"dcs:settledWith", claimed.SettledWith},
		{"dcs:settledAt", claimed.SettledAt},
	} {
		if strings.TrimSpace(required.value) == "" {
			return nil, fmt.Errorf("settlement is missing %s", required.field)
		}
	}

	settledAt, err := time.Parse(time.RFC3339Nano, claimed.SettledAt)
	if err != nil {
		return nil, fmt.Errorf("settlement dcs:settledAt %q is not an RFC3339 timestamp: %w", claimed.SettledAt, err)
	}

	// Re-derive the signed bytes from the claimed fields instead of trusting the
	// sender's serialization: anything the canonical form does not carry — an
	// extra property, a differently written timestamp, a foreign @context —
	// makes the artifact something other than the settlement it reads as.
	expected, err := jades.BuildSettlementPayload(jades.Settlement{
		ContractDID:     claimed.ContractDID,
		ContractVersion: claimed.ContractVersion,
		DocumentDigest:  claimed.DocumentDigest,
		SettledBy:       claimed.SettledBy,
		SettledWith:     claimed.SettledWith,
		SettledAt:       settledAt,
	})
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(payload, expected) {
		return nil, errors.New("settlement payload is not the canonical form of the fields it claims")
	}

	if claimed.ContractDID != local.ContractIRI {
		return nil, fmt.Errorf("settlement binds contract %s, not the shipped contract %s", claimed.ContractDID, local.ContractIRI)
	}
	if !identity.SameDIDWeb(claimed.SettledBy, fromPeerDID) {
		return nil, fmt.Errorf("settlement was made by %s but shipped by %s", claimed.SettledBy, fromPeerDID)
	}
	if !identity.SameDIDWeb(claimed.SettledWith, local.LocalPeer) {
		return nil, fmt.Errorf("settlement was made toward %s, not this instance %s", claimed.SettledWith, local.LocalPeer)
	}
	isParty := false
	for _, party := range local.Parties {
		if identity.SameDIDWeb(party, claimed.SettledBy) {
			isParty = true
			break
		}
	}
	if !isParty {
		return nil, fmt.Errorf("peer %s is not a party of contract %s", claimed.SettledBy, local.ContractIRI)
	}

	// The binding that matters. contract_version is a per-instance counter —
	// the receiver bumps it on every inbound ship, the sender only on merging a
	// redline — so the versions are not comparable across instances and the
	// digest of the document itself is what says "the same version". A
	// settlement of a document this instance does not hold authorizes nothing.
	if claimed.DocumentDigest != local.DocumentDigest {
		return nil, fmt.Errorf("settlement covers document %s but this instance holds %s for contract %s",
			claimed.DocumentDigest, local.DocumentDigest, local.ContractIRI)
	}

	if settledAt.After(local.Now.Add(settlementClockSkew)) {
		return nil, fmt.Errorf("settlement is dated %s, further ahead than the tolerated clock skew", settledAt.Format(time.RFC3339Nano))
	}
	if local.Previous != nil && settledAt.Before(local.Previous.SettledAt) {
		return nil, fmt.Errorf("settlement is dated %s, older than the settlement already held from %s (%s)",
			settledAt.Format(time.RFC3339Nano), fromPeerDID, local.Previous.SettledAt.Format(time.RFC3339Nano))
	}

	received := local.Now
	return &db2.Settlement{
		DID:             local.ContractIRI,
		FromPeerDID:     fromPeerDID,
		ToPeerDID:       local.LocalPeer,
		ContractVersion: claimed.ContractVersion,
		DocumentDigest:  claimed.DocumentDigest,
		SettledAt:       settledAt,
		JadesSignature:  jadesSignature,
		// Evidence in the hands of its audience: the delivery bookkeeping the
		// outbound queue uses is closed for an artifact that has arrived.
		DeliveredAt: &received,
	}, nil
}
