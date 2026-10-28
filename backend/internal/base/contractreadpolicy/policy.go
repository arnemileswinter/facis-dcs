// Package contractreadpolicy shares organization-scoped contract read authorization.
package contractreadpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/userrole"
)

var ErrDenied = errors.New("not authorized to access this contract")

type Scope struct {
	Organization string
	Roles        userrole.UserRoles
	LocalPeer    string
}
type scopeKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}
func FromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}
func (s Scope) Privileged() bool {
	for _, role := range s.Roles {
		switch role {
		case userrole.SystemContractCreator, userrole.SystemContractReviewer, userrole.SystemContractApprover, userrole.SystemContractManager, userrole.SystemContractSigner, userrole.SystemAdministrator, userrole.Auditor:
			return true
		}
	}
	return false
}
func (s Scope) MayRead(origin, createdBy string, raw *datatype.JSON) bool {
	if s.Privileged() || (s.LocalPeer != "" && origin != "" && origin != s.LocalPeer) {
		return true
	}
	if s.Organization == "" {
		return false
	}
	if s.Organization == createdBy {
		return true
	}
	if raw == nil {
		return false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(*raw, &doc) != nil {
		return false
	}
	var parties []json.RawMessage
	if json.Unmarshal(doc["dcs:parties"], &parties) != nil {
		return false
	}
	for _, party := range parties {
		var node map[string]any
		if json.Unmarshal(party, &node) == nil && node["dcs:legalName"] == s.Organization {
			return true
		}
	}
	return false
}

// SQL uses the current trusted contract row, including when listing frozen archive metadata.
// didExpression is a repository-owned SQL identifier, never request input.
func SQL(ctx context.Context, didExpression string, params *[]any) string {
	s, scoped := FromContext(ctx)
	if !scoped || s.Privileged() {
		return "TRUE"
	}
	n := len(*params) + 1
	*params = append(*params, s.Organization, s.LocalPeer)
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM contracts_effective read_contract WHERE read_contract.did = %s AND (
 ($%d <> '' AND read_contract.origin <> '' AND read_contract.origin <> $%d)
 OR ($%d <> '' AND (read_contract.created_by = $%d OR EXISTS (
 SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(read_contract.contract_data->'dcs:parties') = 'array' THEN read_contract.contract_data->'dcs:parties' ELSE '[]'::jsonb END) party
 WHERE jsonb_typeof(party->'dcs:legalName') = 'string' AND party->>'dcs:legalName' = $%d)))))`, didExpression, n+1, n+1, n, n, n)
}
