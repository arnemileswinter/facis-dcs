package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/templaterepository/command"
	pg "digital-contracting-service/internal/templaterepository/db/pg"
)

type echoVCSigner struct{}

func (echoVCSigner) CreateCredential(_ context.Context, unsigned json.RawMessage) (json.RawMessage, error) {
	return unsigned, nil
}

// DCS-IR-TR-06, DCS-FR-TR-14, DCS-FR-TR-17: only an approved template is registered.
func TestTemplateRegisterRequiresApprovedPostgres(t *testing.T) {
	database := pgtest.Open(t)
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

// DCS-FR-TR-17, DCS-FR-TR-18: archive deprecates a live template and deletes a deprecated one.
func TestTemplateArchiveDeletesDeprecatedPostgres(t *testing.T) {
	database := pgtest.Open(t)
	ctx := context.Background()
	archiver := &command.Archiver{DB: database, CTRepo: &pg.PostgresContractTemplateRepo{}, RTRepo: &pg.PostgresReviewTaskRepo{}, ATRepo: &pg.PostgresApprovalTaskRepo{}}
	future := time.Now().Add(time.Hour)
	cases := map[string]string{"REGISTERED": "DEPRECATED", "PUBLISHED": "DEPRECATED", "DEPRECATED": "DELETED", "REVIEWED": "DELETED", "DELETED": ""}
	for state, want := range cases {
		t.Run(state, func(t *testing.T) {
			did := "archive-" + state
			_, err := database.Exec(`INSERT INTO contract_templates (did,created_by,state,template_type,party_type,name) VALUES ($1,'creator',$2,'CONTRACT_TEMPLATE','SINGLE_PARTY','n')`, did, state)
			require.NoError(t, err)
			err = archiver.Handle(ctx, command.ArchiveCmd{DID: did, UpdatedAt: future, ArchivedBy: "manager", UserRoles: userrole.UserRoles{userrole.TemplateManager}})
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
