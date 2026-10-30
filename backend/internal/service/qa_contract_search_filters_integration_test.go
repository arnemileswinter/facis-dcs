package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/contractreadpolicy"
	"digital-contracting-service/internal/base/datatype"
	cwedb "digital-contracting-service/internal/contractworkflowengine/db"
	cwepg "digital-contracting-service/internal/contractworkflowengine/db/pg"
)

// DCS-IR-CWE-07 and DCS-FR-CWE-13: contract search filters by party, template
// and jurisdiction.
func TestQAContractSearchFiltersByPartyTemplateAndJurisdictionPostgres(t *testing.T) {
	database, _ := qaPostgresSchema(t)
	_, err := database.Exec(`
		INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,responsible,contract_data) VALUES
		('de','','Org A','DRAFT','tpl-1',1,'{"creator":"did:web:a","counterparty":"did:web:b"}',
		 '{"dcs:contractFields":[{"@id":"de#j","dcs:shape":{"@id":"https://w3id.org/facis/dcs/taxonomy/v1#field-contract-jurisdiction"},"dcs:value":"Germany"}]}'),
		('fr','','Org A','DRAFT','tpl-2',1,'{"creator":"did:web:a","counterparty":"did:web:c"}',
		 '{"dcs:contractFields":[{"@id":"fr#j","dcs:shape":{"@id":"https://w3id.org/facis/dcs/taxonomy/v1#field-contract-jurisdiction"},"dcs:value":{"@value":"France"}}]}')`)
	require.NoError(t, err)
	ctx := contractreadpolicy.WithSystemScope(context.Background())
	search := func(values cwedb.SearchValues) []string {
		tx, err := database.BeginTxx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		rows, err := (&cwepg.PostgresContractRepo{}).ReadAllMetaDataByFilter(ctx, tx, values, datatype.Pagination{})
		require.NoError(t, err)
		dids := []string{}
		for _, row := range rows {
			dids = append(dids, row.DID)
		}
		return dids
	}

	require.Equal(t, []string{"fr"}, search(cwedb.SearchValues{Party: "did:web:c"}))
	require.Equal(t, []string{"de"}, search(cwedb.SearchValues{TemplateDID: "tpl-1"}))
	require.Equal(t, []string{"de"}, search(cwedb.SearchValues{Jurisdiction: "germ"}))
	require.Equal(t, []string{"fr"}, search(cwedb.SearchValues{Jurisdiction: "France", Party: "did:web:a"}))
}
