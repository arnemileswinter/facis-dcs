package service

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/templatestructure"
	cwecommand "digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/semantichub"
)

// DCS-FR-TR-07: contract creation refuses a template the policy audit at
// approval found NON_COMPLIANT and accepts a COMPLIANT one.
func TestContractCreationRefusesNonCompliantTemplatePostgres(t *testing.T) {
	f := newContractsFromTemplates(t)

	createStructuredTemplate(t, f.creator, "tpl-open", sodTemplateData, templatestructure.Agreement)
	require.Equal(t, "NON_COMPLIANT", f.approveAndRegister("tpl-open"))
	require.ErrorIs(t, f.createContract("c-open", "tpl-open"), cwecommand.ErrTemplateNotCompliant)

	createStructuredTemplate(t, f.creator, "tpl-bound", compliantTemplateData, templatestructure.Agreement)
	require.Equal(t, "COMPLIANT", f.approveAndRegister("tpl-bound"))
	require.NoError(t, f.createContract("c-bound", "tpl-bound"))
}

// DCS-FR-TR-07: the template policy audit knows the domain fields of every
// active hub ontology, not only the seeded one.
func TestDomainOntologyIncludesRegisteredOntologiesPostgres(t *testing.T) {
	f := newContractsFromTemplates(t)
	hosting, err := os.ReadFile("../../../features/fixtures/facis-sla-hosting-ontology.ttl")
	require.NoError(t, err)
	tx := f.database.MustBegin()
	_, err = (semantichub.Repo{}).Register(f.ctx, tx, "facis-sla-hosting", "ontology", "text/turtle", string(hosting), "org", 0, true)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	content, _, err := (semantichub.HubShapeSource{DB: f.database}).ActiveDomainOntology(f.ctx)
	require.NoError(t, err)
	require.Contains(t, content, "field-service-description", "the seeded SLA ontology")
	require.Contains(t, content, "field-service-serviceCreditRate", "the registered hosting ontology")
}
