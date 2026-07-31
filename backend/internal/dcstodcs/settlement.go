package dcstodcs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/base/jades"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	cwedb "digital-contracting-service/internal/contractworkflowengine/db"
	db2 "digital-contracting-service/internal/dcstodcs/db"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"

	"github.com/jmoiron/sqlx"
	"goa.design/clue/log"
)

// SettlementSender delivers one settlement artifact to a counterparty
// instance; injectable so the ship can be driven without a live peer.
type SettlementSender interface {
	SendSettlement(ctx context.Context, peerDID string, req *dcstodcs.DCSToDCSContractSettlementRequest) error
}

// HTTPSettlementSender calls POST /peer/contracts/settlement on the peer
// resolved from its did:web identifier — the same client, path prefixing and
// scheme fallback the PDF ship uses.
type HTTPSettlementSender struct{}

func (HTTPSettlementSender) SendSettlement(ctx context.Context, peerDID string, req *dcstodcs.DCSToDCSContractSettlementRequest) error {
	hostname, segments, err := identity.DIDWebPath(peerDID)
	if err != nil {
		return err
	}
	peerPrefix := ""
	if len(segments) > 0 {
		peerPrefix = "/" + strings.Join(segments, "/")
	}
	client := NewDCSToDCSHttpClient(hostname, peerPrefix)
	_, err = client.PostSettlement(ctx, req)
	return err
}

// BuildSettlement produces the artifact this instance ships to one peer on
// reaching its own settled state: a JAdES over the JCS-canonicalized
// dcs:ContractSettlement node, signed with the same instance key as the
// contract signature ship, binding the document by digest.
func BuildSettlement(doc *identity.DIDDocument, contract *cwedb.Contract, localPeer, peer string, settledAt time.Time) (db2.Settlement, error) {
	contractDocument := []byte(`{}`)
	if contract.ContractData != nil && contract.ContractData.IsNotNullValue() {
		contractDocument = []byte(*contract.ContractData)
	}
	digest, err := jades.ContractDocumentDigest(contractDocument)
	if err != nil {
		return db2.Settlement{}, fmt.Errorf("digest contract document of %s: %w", contract.DID, err)
	}

	// Truncated to what a Postgres timestamp keeps: the stored settled_at must
	// equal the one inside the signature, or a re-delivery of the stored
	// artifact would carry a time this instance no longer holds.
	settledAt = settledAt.UTC().Truncate(time.Microsecond)
	payload, err := jades.BuildSettlementPayload(jades.Settlement{
		ContractDID:     contract.DID,
		ContractVersion: contract.ContractVersion,
		DocumentDigest:  digest,
		SettledBy:       localPeer,
		SettledWith:     peer,
		SettledAt:       settledAt,
	})
	if err != nil {
		return db2.Settlement{}, fmt.Errorf("build settlement payload for %s: %w", contract.DID, err)
	}
	signature, err := jades.Sign(doc, payload)
	if err != nil {
		return db2.Settlement{}, fmt.Errorf("JAdES-sign the settlement of %s: %w", contract.DID, err)
	}

	return db2.Settlement{
		DID:             contract.DID,
		FromPeerDID:     localPeer,
		ToPeerDID:       peer,
		ContractVersion: contract.ContractVersion,
		DocumentDigest:  digest,
		SettledAt:       settledAt,
		JadesSignature:  signature,
	}, nil
}

// shipSettlement records and ships this instance's settlement of a contract to
// every counterparty. The artifact is persisted BEFORE it is delivered and
// only marked delivered once the peer accepted it, so a peer that is down when
// we settle is re-attempted by the retry scheduler instead of the settlement
// being lost — a settlement that never arrives stalls the contract forever,
// since the peer may not sign without it.
//
// Producing the same artifact twice is a no-op: an existing settlement toward
// the same peer for the same document is re-delivered as is, keeping its
// timestamp and signature stable across retries.
func (s *DCSToDCSSynchronizer) shipSettlement(ctx context.Context, did string) error {
	localPeer, err := s.DIDDocument.GetID()
	if err != nil {
		return err
	}
	if s.SettlementSender == nil {
		return fmt.Errorf("could not ship the settlement of %s: no settlement sender is configured", did)
	}

	readTx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	contractData, err := s.CRepo.ReadDataByDID(ctx, readTx, did)
	if err != nil {
		_ = readTx.Rollback()
		return fmt.Errorf("could not read contract %s: %w", did, err)
	}
	_ = readTx.Rollback()

	if contractData.Responsible == nil {
		return nil
	}
	for _, peer := range contractData.Responsible.GetParties() {
		if identity.SameDIDWeb(peer, localPeer) {
			continue
		}
		settlement, err := s.recordSettlement(ctx, contractData, localPeer, peer)
		if err != nil {
			return err
		}
		if settlement.DeliveredAt != nil {
			continue
		}
		if err := s.deliverSettlement(ctx, settlement, true); err != nil {
			return fmt.Errorf("could not deliver the settlement of %s to %s: %w", did, peer, err)
		}
	}
	return nil
}

// recordSettlement returns the stored settlement toward peer for the
// contract's current document, producing and persisting a fresh one when none
// covers that document yet. A renegotiated document is a different digest and
// therefore a new, undelivered artifact.
func (s *DCSToDCSSynchronizer) recordSettlement(ctx context.Context, contract *cwedb.Contract, localPeer, peer string) (db2.Settlement, error) {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return db2.Settlement{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf(ctx, "could not rollback transaction: %v", err)
		}
	}(tx)

	settlement, err := BuildSettlement(&s.DIDDocument, contract, localPeer, peer, time.Now())
	if err != nil {
		return db2.Settlement{}, err
	}

	existing, err := s.SRepo.GetSettlement(ctx, tx, contract.DID, localPeer)
	if err != nil {
		return db2.Settlement{}, fmt.Errorf("could not read the own settlement of %s: %w", contract.DID, err)
	}
	if existing != nil && existing.ToPeerDID == peer && existing.DocumentDigest == settlement.DocumentDigest {
		return *existing, nil
	}

	if err := s.SRepo.UpsertSettlement(ctx, tx, settlement); err != nil {
		return db2.Settlement{}, fmt.Errorf("could not record the settlement of %s: %w", contract.DID, err)
	}
	if err := tx.Commit(); err != nil {
		return db2.Settlement{}, err
	}
	return settlement, nil
}

// deliverSettlement ships one recorded settlement and marks it delivered once
// the peer accepted it. The outbound trust gate applies as it does to a PDF
// ship (ADR-19); its denial is reported as an incident only for the attempt
// the settlement itself triggered, so the retry scheduler re-attempting a
// still-denied peer every interval does not flood the audit trail.
func (s *DCSToDCSSynchronizer) deliverSettlement(ctx context.Context, settlement db2.Settlement, recordIncident bool) error {
	if err := s.TrustGate.Check(ctx, settlement.ToPeerDID, Outbound, settlement.DID, contractstate.Submitted.String()); err != nil {
		var gateErr *GateError
		if recordIncident && errors.As(err, &gateErr) {
			if incidentErr := RecordDenialIncident(ctx, s.DB, settlement.DID, Outbound, gateErr); incidentErr != nil {
				log.Printf(ctx, "could not record trust gate denial incident for %s: %v", settlement.DID, incidentErr)
			}
		}
		return err
	}

	secretValue := rand.Text()
	secretHash, err := s.DIDDocument.Sign([]byte(secretValue))
	if err != nil {
		return err
	}
	if err := s.SettlementSender.SendSettlement(ctx, settlement.ToPeerDID, &dcstodcs.DCSToDCSContractSettlementRequest{
		FromPeerDid:     settlement.FromPeerDID,
		ContractIri:     settlement.DID,
		SecretValue:     secretValue,
		SecretHash:      secretHash,
		SettlementJades: settlement.JadesSignature,
	}); err != nil {
		return err
	}

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf(ctx, "could not rollback transaction: %v", err)
		}
	}(tx)
	if err := s.SRepo.MarkSettlementDelivered(ctx, tx, settlement.DID, settlement.FromPeerDID, settlement.ToPeerDID); err != nil {
		return fmt.Errorf("could not mark the settlement of %s delivered: %w", settlement.DID, err)
	}
	return tx.Commit()
}

// retryUndeliveredSettlements re-ships every settlement this instance produced
// that no peer has accepted yet. Kept out of the sync_fails queue on purpose:
// that queue is keyed by contract and its retry re-ships the PDF, so a failed
// settlement ship recorded there would be retried as a PDF ship and the
// settlement would silently never arrive.
func (s *DCSToDCSSynchronizer) retryUndeliveredSettlements(ctx context.Context) {
	localPeer, err := s.DIDDocument.GetID()
	if err != nil {
		log.Printf(ctx, "could not read the own peer identity for settlement retries: %v", err)
		return
	}

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		log.Printf(ctx, "could not start transaction: %v", err)
		return
	}
	pending, err := s.SRepo.GetUndeliveredSettlements(ctx, tx, localPeer)
	_ = tx.Rollback()
	if err != nil {
		log.Printf(ctx, "could not read undelivered settlements: %v", err)
		return
	}

	for _, settlement := range pending {
		if err := s.deliverSettlement(ctx, settlement, false); err != nil {
			log.Printf(ctx, "settlement ship retry for %s toward %s was not successful: %v",
				settlement.DID, settlement.ToPeerDID, err)
		}
	}
}
