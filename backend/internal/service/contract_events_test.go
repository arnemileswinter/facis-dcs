package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	cwe "digital-contracting-service/gen/contract_workflow_engine"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/contractworkflowengine/changefeed"
	"digital-contracting-service/internal/middleware"
)

type contractEventsRecorder struct {
	sent chan *cwe.ContractChangeEvent
}

func (r *contractEventsRecorder) Send(evt *cwe.ContractChangeEvent) error {
	return r.SendWithContext(context.Background(), evt)
}
func (r *contractEventsRecorder) SendWithContext(_ context.Context, evt *cwe.ContractChangeEvent) error {
	r.sent <- evt
	return nil
}
func (r *contractEventsRecorder) Close() error { return nil }

// DCS-FR-CWE-24: the live stream names a changed contract only to a caller
// that may read it. The fake database serves a contract created by Org A with
// Org B as a party.
func TestContractEventsFollowTheReadPolicy(t *testing.T) {
	database := sqlx.NewDb(sql.OpenDB(lifecycleContractConnector{}), "postgres")
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	feed := changefeed.New()
	engine := &contractWorkflowEnginesrvc{DB: database, DIDDocument: *jadesTestDIDDocument(t, "local"), Changes: feed}

	open := func(org string) (*contractEventsRecorder, context.CancelFunc, chan error) {
		ctx, cancel := context.WithCancel(middleware.InjectAuthContext(context.Background(), []string{userrole.ContractObserver.String()}, "holder", org))
		recorder := &contractEventsRecorder{sent: make(chan *cwe.ContractChangeEvent, 8)}
		done := make(chan error, 1)
		go func() { done <- engine.Events(ctx, &cwe.ContractEventsRequest{}, recorder) }()
		require.Equal(t, "CONNECTED", (<-recorder.sent).EventType)
		return recorder, cancel, done
	}
	party, stopParty, partyDone := open("Org B")
	foreign, stopForeign, foreignDone := open("Org C")

	feed.Publish(changefeed.Change{DID: "did:contract:1", EventType: "SUBMIT_CONTRACT"})
	require.Equal(t, &cwe.ContractChangeEvent{Did: "did:contract:1", EventType: "SUBMIT_CONTRACT"}, <-party.sent)

	require.Never(t, func() bool { return len(foreign.sent) > 0 }, 200*time.Millisecond, 10*time.Millisecond,
		"a foreign organization must not learn of the change")

	stopParty()
	stopForeign()
	require.NoError(t, <-partyDone)
	require.NoError(t, <-foreignDone)
}
