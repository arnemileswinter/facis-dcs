package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/contractworkflowengine/command"
	cwepg "digital-contracting-service/internal/contractworkflowengine/db/pg"
	trustgate "digital-contracting-service/internal/dcstodcs"
	syncpg "digital-contracting-service/internal/dcstodcs/db/pg"
)

type capturingTerminationSender struct {
	sent []*dcstodcs.DCSToDCSContractTerminateRequest
	fail error
}

func (s *capturingTerminationSender) SendTermination(_ context.Context, _ string, req *dcstodcs.DCSToDCSContractTerminateRequest) error {
	s.sent = append(s.sent, req)
	return s.fail
}

// DCS-NFR-BR-06: a local termination queues a notice toward the counterparty
// instance in the same transaction, and the notifier delivers it until the
// peer confirms. The signed secret names the contract.
func TestQATerminationIsPropagatedToThePeerPostgres(t *testing.T) {
	database := pgtest.Open(t)
	document := jadesTestDIDDocument(t, "local")
	peer, err := document.GetID()
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,responsible) VALUES ('c1',$1,'Org A','ACTIVE','tpl',1,
		jsonb_build_object('creator',$2::text,'counterparty','did:web:peer.example'))`, peer, peer)
	require.NoError(t, err)

	handler := command.Terminator{DB: database, CRepo: &cwepg.PostgresContractRepo{}, SRepo: syncpg.PostgresSyncRepository{}, DIDDocument: *document}
	require.NoError(t, handler.Handle(context.Background(), command.TerminateCmd{
		DID: "c1", TerminatedBy: "Org A", Reason: "breach", UpdatedAt: time.Now().Add(time.Hour), CauserDID: peer,
		UserRoles: userrole.UserRoles{userrole.ContractManager},
	}))

	sender := &capturingTerminationSender{fail: errors.New("peer down")}
	notifier := trustgate.TerminationNotifier{DB: database, DIDDocument: *document, Sender: sender}
	notifier.DeliverPending(context.Background())
	require.Len(t, sender.sent, 1)
	require.Equal(t, "c1", sender.sent[0].ContractIri)
	require.Equal(t, "breach", *sender.sent[0].Reason)
	require.True(t, strings.HasPrefix(sender.sent[0].SecretValue, trustgate.TerminationNoticePrefix("c1")))

	var retries int
	require.NoError(t, database.Get(&retries, `SELECT retry_count FROM contract_peer_terminations WHERE did='c1' AND peer_did='did:web:peer.example' AND confirmed_at IS NULL`))
	require.Equal(t, 1, retries)

	_, err = database.Exec(`UPDATE contract_peer_terminations SET last_tried_at = now() - INTERVAL '1 hour'`)
	require.NoError(t, err)
	sender.fail = nil
	notifier.DeliverPending(context.Background())
	var confirmed int
	require.NoError(t, database.Get(&confirmed, `SELECT count(*) FROM contract_peer_terminations WHERE confirmed_at IS NOT NULL`))
	require.Equal(t, 1, confirmed)
}

// The receiving side terminates its copy only on notice from a party, does not
// notify back, and confirms a repeated notice.
func TestQAPeerTerminationNoticeTerminatesTheLocalCopyPostgres(t *testing.T) {
	database := pgtest.Open(t)
	document := jadesTestDIDDocument(t, "local")
	local, err := document.GetID()
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,responsible) VALUES ('c1','did:web:peer.example','Org B','ACTIVE','tpl',1,
		jsonb_build_object('creator','did:web:peer.example','counterparty',$1::text))`, local)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contract_review_task (did,state,reviewer,created_by) VALUES ('c1','OPEN','r1','Org B')`)
	require.NoError(t, err)
	reason := "breach"
	notice := func(from string) *dcstodcs.DCSToDCSContractTerminateRequest {
		return &dcstodcs.DCSToDCSContractTerminateRequest{FromPeerDid: from, ContractIri: "c1", Reason: &reason}
	}
	terminate := func(from string) error {
		return terminateForPeer(context.Background(), database, &cwepg.PostgresContractRepo{}, syncpg.PostgresSyncRepository{}, *document, notice(from))
	}

	require.Error(t, terminate("did:web:stranger.example"))
	var state string
	require.NoError(t, database.Get(&state, `SELECT state FROM contracts WHERE did='c1'`))
	require.Equal(t, "ACTIVE", state)

	require.NoError(t, terminate("did:web:peer.example"))
	require.NoError(t, database.Get(&state, `SELECT state FROM contracts WHERE did='c1'`))
	require.Equal(t, "TERMINATED", state)
	var open, queued int
	require.NoError(t, database.Get(&open, `SELECT count(*) FROM contract_review_task WHERE state='OPEN'`))
	require.Zero(t, open)
	require.NoError(t, database.Get(&queued, `SELECT count(*) FROM contract_peer_terminations`))
	require.Zero(t, queued, "a received termination is not sent back")

	require.NoError(t, terminate("did:web:peer.example"))
}

// A notice whose signed secret names another contract is refused before any
// peer lookup.
func TestPeerTerminationNoticeMustNameTheContract(t *testing.T) {
	s := &dcsToDcssrvc{}
	_, err := s.Terminate(context.Background(), &dcstodcs.DCSToDCSContractTerminateRequest{
		FromPeerDid: "did:web:peer.example", ContractIri: "c1", SecretValue: trustgate.TerminationNoticePrefix("c2") + "nonce",
	})
	require.ErrorContains(t, err, "does not name this contract")
}
