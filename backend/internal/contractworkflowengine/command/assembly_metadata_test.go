package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/contractworkflowengine/db"
)

// DCS-FR-CWE-03: an assembled contract leaves draft only with its required
// metadata.
func TestAssemblyRequiresAContractName(t *testing.T) {
	blank := "  "
	named := "Service Agreement"
	for _, tc := range []struct {
		name *string
		ok   bool
	}{{nil, false}, {&blank, false}, {&named, true}} {
		repo := &submitContractRepoFake{stored: &db.Contract{DID: "c1", Name: tc.name}}
		err := requireAssemblyMetadata(context.Background(), nil, repo, "c1")
		if tc.ok {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, ErrMissingRequiredMetadata)
		}
	}
}
