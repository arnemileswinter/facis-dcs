package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/semantichub"
	"digital-contracting-service/internal/templaterepository/command"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatetype"
	pg "digital-contracting-service/internal/templaterepository/db/pg"
	"digital-contracting-service/internal/templaterepository/query/contracttemplate"
)

// DCS-FR-TR-10: templates are searchable by jurisdiction, industry and the
// regulatory compliance status the policy audit sets at approval.
func TestTemplateSearchableMetadataPostgres(t *testing.T) {
	database := pgtest.Open(t)
	ctx := context.Background()
	require.NoError(t, semantichub.Seed(ctx, database))
	validation.SetShapeSource(semantichub.HubShapeSource{DB: database})
	ctRepo := &pg.PostgresContractTemplateRepo{}

	create := func(did, jurisdiction, industry string) {
		t.Helper()
		data := datatype.JSON(sodTemplateData)
		require.NoError(t, (&command.Creator{DB: database, CTRepo: ctRepo}).Handle(ctx, command.CreateCmd{
			DID: did, CreatedBy: "org", TemplateType: contracttemplatetype.ContractTemplate, PartyType: "SINGLE_PARTY",
			TemplateData: &data, Jurisdiction: &jurisdiction, Industry: &industry,
			UserRoles: userrole.UserRoles{userrole.TemplateCreator},
		}))
	}
	create("tpl-de-energy", "DEU", "Energy")
	create("tpl-fr-cloud", "FRA", "Cloud hosting")

	_, err := database.Exec(`UPDATE contract_templates SET state = 'REVIEWED' WHERE did = 'tpl-de-energy'`)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO contract_templates_approval_task (did,state,approver,created_by) VALUES ('tpl-de-energy','OPEN','org','org')`)
	require.NoError(t, err)
	require.NoError(t, (&command.Approver{DB: database, CTRepo: ctRepo, ATRepo: &pg.PostgresApprovalTaskRepo{}}).Handle(ctx, command.ApproveCmd{
		DID: "tpl-de-energy", UpdatedAt: time.Now().Add(time.Hour), ApprovedBy: "org", UserRoles: userrole.UserRoles{userrole.TemplateApprover},
	}))

	search := func(qry contracttemplate.GetAllMetadataByFilterQry) []string {
		t.Helper()
		result, err := (&contracttemplate.GetAllMetaDataByFilterHandler{DB: database, CTRepo: ctRepo}).Handle(ctx, qry)
		require.NoError(t, err)
		dids := []string{}
		for _, item := range result {
			dids = append(dids, item.DID)
		}
		return dids
	}
	require.Equal(t, []string{"tpl-de-energy"}, search(contracttemplate.GetAllMetadataByFilterQry{Jurisdiction: "deu"}))
	require.Equal(t, []string{"tpl-fr-cloud"}, search(contracttemplate.GetAllMetadataByFilterQry{Industry: "cloud"}))
	// The minimal template has no clause bound to contract data, an error finding of the policy audit.
	require.Equal(t, []string{"tpl-de-energy"}, search(contracttemplate.GetAllMetadataByFilterQry{ComplianceStatus: "NON_COMPLIANT"}))
	require.Equal(t, []string{"tpl-fr-cloud"}, search(contracttemplate.GetAllMetadataByFilterQry{ComplianceStatus: "UNASSESSED"}))
}

func TestTemplateComplianceStatus(t *testing.T) {
	require.Equal(t, "COMPLIANT", command.ComplianceStatus([]validation.PolicyFinding{{Severity: validation.SeverityWarning}}))
	require.Equal(t, "NON_COMPLIANT", command.ComplianceStatus([]validation.PolicyFinding{{Severity: validation.SeverityWarning}, {Severity: validation.SeverityError}}))
}
