package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTmpSLAContextNormalization(t *testing.T) {
	defer slaHostingShapeSource(t)()
	defer SetSchemaAnchorRefs(SchemaJSONLDContextV1, SchemaSHACLShapesV1)
	SetSchemaAnchorRefs("http://dcs-a.localhost:18080/digital-contracting-service/api/semantic/context/facis-dcs?version=1", SchemaSHACLShapesV1)
	SetCanonicalOntologyIRIs(map[string]string{
		"dcs":  "https://w3id.org/facis/dcs/ontology/v1#",
		"odrl": "http://www.w3.org/ns/odrl/2/",
		"sh":   "http://www.w3.org/ns/shacl#",
		"xsd":  "http://www.w3.org/2001/XMLSchema#",
	})
	defer SetCanonicalOntologyIRIs(nil)
	doc := slaHostingTemplate(t)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	out, err := NormalizeTemplateData(string(raw))
	require.NoError(t, err)
	var normalized map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &normalized))
	ctx, _ := json.Marshal(normalized["@context"])
	t.Fatalf("normalized @context = %s", ctx)
}
