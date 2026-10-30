package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/templaterepository/command"
	pg "digital-contracting-service/internal/templaterepository/db/pg"
)

type echoVCSigner struct{}

func (echoVCSigner) CreateCredential(_ context.Context, unsigned json.RawMessage) (json.RawMessage, error) {
	return unsigned, nil
}

// DCS-IR-TR-06, DCS-FR-TR-14, DCS-FR-TR-17: only an approved template is registered.
func TestTemplateRegisterRequiresApprovedPostgres(t *testing.T) {
	database, _ := qaPostgresSchema(t)
	ctx := context.Background()
	registrar := &command.Registrar{
		DB: database, CTRepo: &pg.PostgresContractTemplateRepo{}, RTRepo: &pg.PostgresReviewTaskRepo{}, ATRepo: &pg.PostgresApprovalTaskRepo{},
		VCSigner: echoVCSigner{}, IssuerDID: "did:web:issuer.example",
	}
	cases := map[string]string{
		"DRAFT": "", "SUBMITTED": "", "REJECTED": "", "REVIEWED": "", "PUBLISHED": "", "DEPRECATED": "", "DELETED": "",
		"APPROVED": "REGISTERED", "REGISTERED": "REGISTERED",
	}
	for state, want := range cases {
		t.Run(state, func(t *testing.T) {
			did := "register-" + state
			_, err := database.Exec(`INSERT INTO contract_templates (did,created_by,state,template_type,party_type,name,template_data) VALUES ($1,'creator',$2,'CONTRACT_TEMPLATE','SINGLE_PARTY','n','{}')`, did, state)
			require.NoError(t, err)
			_, err = registrar.Handle(ctx, command.RegisterCmd{DID: did, RegisteredBy: "manager", UserRoles: userrole.UserRoles{userrole.TemplateManager}})
			var stored string
			require.NoError(t, database.Get(&stored, `SELECT state FROM contract_templates WHERE did=$1`, did))
			if want == "" {
				require.ErrorIs(t, err, command.ErrInvalidTemplateState)
				require.Equal(t, state, stored)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, stored)
		})
	}
}
