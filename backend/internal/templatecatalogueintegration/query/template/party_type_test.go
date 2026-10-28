package template

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/templatecatalogueintegration/client"
)

// fakeCatalogue returns fixed rows; the package has no SPARQL evaluator to run the query text against.
func fakeCatalogue(t *testing.T, rows []map[string]interface{}) *client.FederatedCatalogueClient {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/token") {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 300, "token_type": "Bearer"}))
			return
		}
		var body struct{ Statement string }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		items := rows
		if strings.Contains(body.Statement, "COUNT(") {
			items = []map[string]interface{}{{"total": len(rows)}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(client.QueryResults{TotalCount: len(items), Items: items}))
	}))
	t.Cleanup(server.Close)
	fc, err := client.NewFederatedCatalogueClient(client.Config{APIURL: server.URL, KeycloakRealmURL: server.URL + "/realms/test", ClientID: "test", ClientSecret: "test"})
	require.NoError(t, err)
	return fc
}

func TestUnclassifiedCatalogueTemplatesCountAsMultiParty(t *testing.T) {
	ctx := context.Background()
	rows := []map[string]interface{}{
		{"did": "single", "version": "1", "party_type": "SINGLE_PARTY", "template_type": "COMPONENT"},
		{"did": "legacy", "version": "1", "template_data_string": "{}"},
	}
	fc := fakeCatalogue(t, rows)

	list, err := (&GetAllMetadataHandler{FCClient: fc}).Handle(ctx, GetAllMetadataQry{})
	require.NoError(t, err)
	require.Len(t, list.Items, 2)
	require.Equal(t, "SINGLE_PARTY", *list.Items[0].PartyType)
	require.Equal(t, "COMPONENT", *list.Items[0].TemplateType)
	require.Equal(t, "MULTI_PARTY", *list.Items[1].PartyType)

	found, err := (&SearchHandler{FCClient: fc}).Handle(ctx, SearchQry{})
	require.NoError(t, err)
	require.Equal(t, "MULTI_PARTY", *found.Items[1].PartyType)

	detail, err := (&GetByIDHandler{FCClient: fakeCatalogue(t, rows[1:])}).Handle(ctx, GetByIDQry{DID: "legacy", Version: 1})
	require.NoError(t, err)
	require.Equal(t, "MULTI_PARTY", *detail.PartyType)
}
