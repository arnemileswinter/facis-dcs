package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/contractworkflowengine/command"
	cwepg "digital-contracting-service/internal/contractworkflowengine/db/pg"
	syncpg "digital-contracting-service/internal/dcstodcs/db/pg"
)

// DCS-FR-CWE-23: termination removes the contract from active workflows. Open
// tasks go; decided tasks stay as the record of the decision.
func TestQATerminateClosesOpenTasksPostgres(t *testing.T) {
	database, _ := qaPostgresSchema(t)
	document := jadesTestDIDDocument(t, "local")
	peer, err := document.GetID()
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version) VALUES ('c1',$1,'Org A','SUBMITTED','tpl',1)`, peer)
	require.NoError(t, err)
	_, err = database.Exec(`
		INSERT INTO contract_review_task (did,state,reviewer,created_by) VALUES ('c1','OPEN','r1','Org A'),('c1','APPROVED','r2','Org A');
		INSERT INTO contract_approval_task (did,state,approver,created_by) VALUES ('c1','OPEN','a1','Org A');
		INSERT INTO contract_negotiation_task (did,contract_version,state,negotiator,created_by) VALUES ('c1',1,'OPEN','n1','Org A')`)
	require.NoError(t, err)

	handler := command.Terminator{DB: database, CRepo: &cwepg.PostgresContractRepo{}, SRepo: syncpg.PostgresSyncRepository{}, DIDDocument: *document}
	require.NoError(t, handler.Handle(context.Background(), command.TerminateCmd{
		DID: "c1", TerminatedBy: "Org A", Reason: "ended", UpdatedAt: time.Now().Add(time.Hour), CauserDID: peer,
		UserRoles: userrole.UserRoles{userrole.ContractManager},
	}))

	var open, decided int
	require.NoError(t, database.Get(&open, `SELECT
		(SELECT count(*) FROM contract_review_task WHERE state='OPEN') +
		(SELECT count(*) FROM contract_approval_task WHERE state='OPEN') +
		(SELECT count(*) FROM contract_negotiation_task WHERE state='OPEN')`))
	require.Zero(t, open)
	require.NoError(t, database.Get(&decided, `SELECT count(*) FROM contract_review_task WHERE state='APPROVED'`))
	require.Equal(t, 1, decided)
}
