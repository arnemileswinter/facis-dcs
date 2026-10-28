package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	"digital-contracting-service/internal/base/contractreadpolicy"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/middleware"
)

func contractReadContext(ctx context.Context, document identity.DIDDocument) (context.Context, error) {
	peer, err := document.GetID()
	if err != nil {
		return ctx, contractworkflowengine.MakeInternalError(err)
	}
	return contractreadpolicy.WithScope(ctx, contractreadpolicy.Scope{Organization: middleware.GetParticipantID(ctx), Roles: middleware.GetUserRoles(ctx), LocalPeer: peer}), nil
}

// authorizeContractRead checks current trusted metadata before any derived read or read-side event.
func authorizeContractRead(ctx context.Context, database *sqlx.DB, localPeer, did string) error {
	var row struct {
		Origin    string         `db:"origin"`
		CreatedBy string         `db:"created_by"`
		Data      *datatype.JSON `db:"contract_data"`
	}
	err := database.GetContext(ctx, &row, `SELECT COALESCE(origin, '') AS origin, created_by, contract_data FROM contracts_effective WHERE did = $1`, did)
	if errors.Is(err, sql.ErrNoRows) {
		return contractworkflowengine.MakeNotFound(err)
	}
	if err != nil {
		return contractworkflowengine.MakeInternalError(err)
	}
	scope := contractreadpolicy.Scope{Organization: middleware.GetParticipantID(ctx), Roles: middleware.GetUserRoles(ctx), LocalPeer: localPeer}
	if !scope.MayRead(row.Origin, row.CreatedBy, row.Data) {
		return contractworkflowengine.MakeForbidden(contractreadpolicy.ErrDenied)
	}
	return nil
}

func authorizeContractReadDocument(ctx context.Context, database *sqlx.DB, document identity.DIDDocument, did string) error {
	peer, err := document.GetID()
	if err != nil {
		return contractworkflowengine.MakeInternalError(err)
	}
	return authorizeContractRead(ctx, database, peer, did)
}

// readableContractDIDs is nil only for the existing privileged cross-organization roles.
func readableContractDIDs(ctx context.Context, database *sqlx.DB) (map[string]bool, error) {
	scope, ok := contractreadpolicy.FromContext(ctx)
	if !ok {
		return nil, contractworkflowengine.MakeForbidden(contractreadpolicy.ErrDenied)
	}
	if scope.Privileged() {
		return nil, nil
	}
	var params []any
	query := "SELECT did FROM contracts_effective WHERE " + contractreadpolicy.SQL(ctx, "contracts_effective.did", &params)
	var dids []string
	if err := database.SelectContext(ctx, &dids, query, params...); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	allowed := make(map[string]bool, len(dids))
	for _, did := range dids {
		allowed[did] = true
	}
	return allowed, nil
}

func filterContractAuditChains(chains [][]datatype.AuditLogEntry, allowed map[string]bool) [][]datatype.AuditLogEntry {
	if allowed == nil {
		return chains
	}
	filtered := make([][]datatype.AuditLogEntry, 0, len(chains))
	for _, chain := range chains {
		entries := make([]datatype.AuditLogEntry, 0, len(chain))
		for _, entry := range chain {
			if entry.DID != nil && allowed[*entry.DID] {
				entries = append(entries, entry)
			}
		}
		if len(entries) > 0 {
			filtered = append(filtered, entries)
		}
	}
	return filtered
}
