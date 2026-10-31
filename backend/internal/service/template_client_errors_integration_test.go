package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	templaterepository "digital-contracting-service/gen/template_repository"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/middleware"
	"digital-contracting-service/internal/templaterepository/command"
	pg "digital-contracting-service/internal/templaterepository/db/pg"
)

// DCS-IR-TR-02, DCS-IR-TR-08: a page size without an offset reads the first
// page, and the search result names the creator.
func TestTemplateSearchFirstPageAndCreatorPostgres(t *testing.T) {
	database := pgtest.Open(t)
	_, err := database.Exec(`INSERT INTO contract_templates (did,created_by,state,template_type,party_type,name) VALUES ('tpl','did:web:creator.example','DRAFT','CONTRACT_TEMPLATE','SINGLE_PARTY','n')`)
	require.NoError(t, err)
	svc := &templateRepositorysrvc{DB: database, CTRepo: &pg.PostgresContractTemplateRepo{}, RTRepo: &pg.PostgresReviewTaskRepo{}, ATRepo: &pg.PostgresApprovalTaskRepo{}}
	ctx := middleware.InjectAuthContext(context.Background(), []string{userrole.TemplateManager.String()}, "did:key:holder", "did:web:org.example")
	limit := 5

	found, err := svc.Search(ctx, &templaterepository.ContractTemplateSearchRequest{Limit: &limit})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "did:web:creator.example", found[0].CreatedBy)

	listed, err := svc.Retrieve(ctx, &templaterepository.ContractTemplateRetrieveRequest{Limit: &limit})
	require.NoError(t, err)
	require.Len(t, listed.ContractTemplates, 1)
}

// DCS-IR-TR-04: input errors of the caller are answered with 400, not 500.
func TestTemplateClientErrorsAreBadRequests(t *testing.T) {
	svc := &templateRepositorysrvc{}
	ctx := middleware.InjectAuthContext(context.Background(), []string{userrole.TemplateReviewer.String()}, "did:key:holder", "did:web:org.example")
	state, flag := "NOT_A_STATE", "elsewhere"

	_, err := svc.Search(ctx, &templaterepository.ContractTemplateSearchRequest{State: &state})
	require.Equal(t, "bad_request", errorName(err))
	_, err = svc.Submit(ctx, &templaterepository.ContractTemplateSubmitRequest{Did: "tpl", UpdatedAt: "yesterday"})
	require.Equal(t, "bad_request", errorName(err))
	_, err = svc.Submit(ctx, &templaterepository.ContractTemplateSubmitRequest{Did: "tpl", UpdatedAt: time.Now().Format(time.RFC3339), ForwardTo: &flag})
	require.Equal(t, "bad_request", errorName(err))
	_, err = svc.Archive(ctx, &templaterepository.ContractTemplateArchiveRequest{Did: "tpl", UpdatedAt: "-", Justification: "x"})
	require.Equal(t, "bad_request", errorName(err))
}

// DCS-IR-TR-04: a command the template's state or the caller's role does not
// allow is a client error.
func TestTemplateCommandRefusalsAreBadRequestsPostgres(t *testing.T) {
	database := pgtest.Open(t)
	ctx := context.Background()
	ctRepo := &pg.PostgresContractTemplateRepo{}
	data := datatype.JSON(sodTemplateData)
	require.NoError(t, (&command.Creator{DB: database, CTRepo: ctRepo}).Handle(ctx, command.CreateCmd{
		DID: "tpl", CreatedBy: "org", TemplateType: "CONTRACT_TEMPLATE", PartyType: "SINGLE_PARTY",
		TemplateData: &data, UserRoles: userrole.UserRoles{userrole.TemplateCreator},
	}))
	future := time.Now().Add(time.Hour)

	err := (&command.Approver{DB: database, CTRepo: ctRepo, ATRepo: &pg.PostgresApprovalTaskRepo{}}).Handle(ctx, command.ApproveCmd{DID: "tpl", UpdatedAt: future, ApprovedBy: "org"})
	require.Equal(t, "bad_request", errorName(mapTemplateCommandError(err)), err)
	err = (&command.Submitter{DB: database, CTRepo: ctRepo, RTRepo: &pg.PostgresReviewTaskRepo{}, ATRepo: &pg.PostgresApprovalTaskRepo{}}).Handle(ctx, command.SubmitCmd{DID: "tpl", UpdatedAt: future, SubmittedBy: "org", UserRoles: userrole.UserRoles{userrole.TemplateApprover}})
	require.Equal(t, "bad_request", errorName(mapTemplateCommandError(err)), err)
}
