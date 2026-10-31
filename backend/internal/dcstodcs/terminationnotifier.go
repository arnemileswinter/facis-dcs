package dcstodcs

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"goa.design/clue/log"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
	basedb "digital-contracting-service/internal/base/db"
	"digital-contracting-service/internal/base/identity"
)

// TerminationNoticePrefix is the start of the signed secret_value of a
// termination notice. It binds the signature to the contract, so a captured
// notice cannot terminate a different contract.
func TerminationNoticePrefix(contractIRI string) string {
	return "terminate|" + contractIRI + "|"
}

// PeerTerminationSender delivers one termination notice to a counterparty
// instance.
type PeerTerminationSender interface {
	SendTermination(ctx context.Context, peerDID string, req *dcstodcs.DCSToDCSContractTerminateRequest) error
}

// HTTPPeerTerminationSender calls POST /peer/contracts/terminate on the peer
// resolved from its did:web identifier.
type HTTPPeerTerminationSender struct{}

func (HTTPPeerTerminationSender) SendTermination(ctx context.Context, peerDID string, req *dcstodcs.DCSToDCSContractTerminateRequest) error {
	hostname, segments, err := identity.DIDWebPath(peerDID)
	if err != nil {
		return err
	}
	peerPrefix := ""
	if len(segments) > 0 {
		peerPrefix = "/" + strings.Join(segments, "/")
	}
	_, err = NewDCSToDCSHttpClient(hostname, peerPrefix).Terminate(ctx, req)
	return err
}

// TerminationNotifier delivers the pending rows of contract_peer_terminations
// (DCS-NFR-BR-06). The terminate command writes a row per counterparty in the
// transaction that terminates the contract; a row stays pending until the peer
// confirms.
type TerminationNotifier struct {
	DB          *sqlx.DB
	DIDDocument identity.DIDDocument
	Sender      PeerTerminationSender
}

type pendingTermination struct {
	DID     string `db:"did"`
	PeerDID string `db:"peer_did"`
	Reason  string `db:"reason"`
}

// DeliverPending sends every unconfirmed notice that is due. A failed notice
// waits five seconds longer per attempt, up to five minutes.
func (n *TerminationNotifier) DeliverPending(ctx context.Context) {
	var pending []pendingTermination
	if err := n.DB.SelectContext(ctx, &pending,
		`SELECT did, peer_did, reason FROM contract_peer_terminations
		  WHERE confirmed_at IS NULL
		    AND (last_tried_at IS NULL OR last_tried_at < now() - LEAST(retry_count, 60) * INTERVAL '5 seconds')
		  ORDER BY requested_at`); err != nil {
		log.Printf(ctx, "could not read pending peer terminations: %v", err)
		return
	}
	localPeer, err := n.DIDDocument.GetID()
	if err != nil {
		log.Printf(ctx, "could not resolve the local DID for peer terminations: %v", err)
		return
	}
	for _, notice := range pending {
		secretValue := TerminationNoticePrefix(notice.DID) + rand.Text()
		secretHash, err := n.DIDDocument.Sign([]byte(secretValue))
		if err == nil {
			err = n.Sender.SendTermination(ctx, notice.PeerDID, &dcstodcs.DCSToDCSContractTerminateRequest{
				FromPeerDid: localPeer,
				ContractIri: notice.DID,
				Reason:      &notice.Reason,
				SecretValue: secretValue,
				SecretHash:  secretHash,
			})
		}
		if err != nil {
			log.Printf(ctx, "termination notice for %s toward %s not delivered, retry pending: %v", notice.DID, notice.PeerDID, err)
			if _, dbErr := n.DB.ExecContext(ctx, `UPDATE contract_peer_terminations SET retry_count = retry_count + 1, last_tried_at = now(), last_error = $3 WHERE did = $1 AND peer_did = $2`,
				notice.DID, notice.PeerDID, err.Error()); dbErr != nil {
				log.Printf(ctx, "could not record termination attempt for %s toward %s: %v", notice.DID, notice.PeerDID, dbErr)
			}
			continue
		}
		if _, err := n.DB.ExecContext(ctx, `UPDATE contract_peer_terminations SET confirmed_at = now(), last_error = NULL WHERE did = $1 AND peer_did = $2`,
			notice.DID, notice.PeerDID); err != nil {
			log.Printf(ctx, "could not confirm termination notice for %s toward %s: %v", notice.DID, notice.PeerDID, err)
		}
	}
}

// Start delivers pending notices on the given interval.
func (n *TerminationNotifier) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := basedb.RunExclusive(ctx, n.DB, "peer-termination-notices", func() error { n.DeliverPending(ctx); return nil }); err != nil {
					log.Printf(ctx, "could not deliver pending termination notices: %v", err)
				}
			}
		}
	}()
}
