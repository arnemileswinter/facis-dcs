package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	goa "goa.design/goa/v3/pkg"

	cwe "digital-contracting-service/gen/contract_workflow_engine"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/middleware"
)

// A contract mutation by an organization that may not read the contract is
// refused before any command runs. The service has no repositories here, so a
// call that passed the check would panic.
func TestQAContractWritesRequireReadAccessPostgres(t *testing.T) {
	database := pgtest.Open(t)
	document := jadesTestDIDDocument(t, "local")
	peer, err := document.GetID()
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,name,contract_data) VALUES ('org-a-draft',$1,'Org A','DRAFT','template',1,'draft','{}')`, peer)
	require.NoError(t, err)

	s := &contractWorkflowEnginesrvc{DB: database, DIDDocument: *document}
	roles := []string{"Contract Creator", "Contract Negotiator", "Contract Reviewer", "Contract Approver", "Contract Manager"}
	ctx := middleware.InjectAuthContext(context.Background(), roles, "holder-c", "Org C")
	did := "org-a-draft"
	calls := map[string]func() error{
		"update":       func() error { _, err := s.Update(ctx, &cwe.ContractUpdateRequest{Did: did}); return err },
		"submit":       func() error { _, err := s.Submit(ctx, &cwe.ContractSubmitRequest{Did: did}); return err },
		"negotiate":    func() error { _, err := s.Negotiate(ctx, &cwe.ContractNegotiationRequest{Did: did}); return err },
		"accept_offer": func() error { _, err := s.AcceptOffer(ctx, &cwe.ContractOfferAcceptRequest{Did: did}); return err },
		"save_negotiation_draft": func() error {
			_, err := s.SaveNegotiationDraft(ctx, &cwe.ContractNegotiationDraftSaveRequest{Did: did})
			return err
		},
		"delete_negotiation_draft": func() error {
			_, err := s.DeleteNegotiationDraft(ctx, &cwe.ContractNegotiationDraftRetrieveRequest{Did: did})
			return err
		},
		"respond":   func() error { _, err := s.Respond(ctx, &cwe.ContractNegotiationRespondRequest{Did: did}); return err },
		"approve":   func() error { _, err := s.Approve(ctx, &cwe.ContractApproveRequest{Did: did}); return err },
		"reject":    func() error { _, err := s.Reject(ctx, &cwe.ContractRejectRequest{Did: did}); return err },
		"terminate": func() error { _, err := s.Terminate(ctx, &cwe.ContractTerminateRequest{Did: did}); return err },
		"renew":     func() error { _, err := s.Renew(ctx, &cwe.ContractRenewRequest{Did: did}); return err },
		"offer":     func() error { _, err := s.Offer(ctx, &cwe.ContractOfferRequest{Did: did}); return err },
		"withdraw":  func() error { _, err := s.Withdraw(ctx, &cwe.ContractWithdrawRequest{Did: did}); return err },
		"deploy":    func() error { _, err := s.Deploy(ctx, &cwe.ContractDeployRequest{Did: did}); return err },
		"designate_target": func() error {
			_, err := s.DesignateContractTarget(ctx, &cwe.ContractTargetDesignateRequest{Did: did})
			return err
		},
	}
	for name, call := range calls {
		var refused *goa.ServiceError
		err := call()
		require.True(t, errors.As(err, &refused) && refused.Name == "forbidden", "%s by a foreign organization: %v", name, err)
	}
}
