package contractreadpolicy

import (
	"context"
	"testing"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/userrole"
)

const localPeer = "did:web:local"

func TestMayRead(t *testing.T) {
	party := datatype.JSON(`{"dcs:parties":[{"dcs:legalName":"Org A"}]}`)
	none := datatype.JSON(`{}`)
	user := Scope{Organization: "Org A", Roles: userrole.UserRoles{userrole.ContractObserver}, LocalPeer: localPeer}
	for _, tc := range []struct {
		name              string
		scope             Scope
		origin, createdBy string
		data              *datatype.JSON
		want              bool
	}{
		{"own organization", user, localPeer, "Org A", &none, true},
		{"named party", user, localPeer, "Org B", &party, true},
		{"foreign organization", user, localPeer, "Org B", &none, false},
		{"foreign without data", user, localPeer, "Org B", nil, false},
		{"adopted from peer", user, "did:web:peer", "Org B", &none, true},
		{"no organization", Scope{Roles: user.Roles, LocalPeer: localPeer}, localPeer, "", &party, false},
		{"system scope", Scope{system: true}, localPeer, "Org B", &none, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.MayRead(tc.origin, tc.createdBy, tc.data); got != tc.want {
				t.Fatalf("MayRead = %v, want %v", got, tc.want)
			}
		})
	}
	for _, role := range []userrole.UserRole{userrole.SystemContractCreator, userrole.SystemContractReviewer, userrole.SystemContractApprover, userrole.SystemContractManager, userrole.SystemContractSigner, userrole.SystemAdministrator, userrole.Auditor} {
		if !(Scope{Roles: userrole.UserRoles{role}, LocalPeer: localPeer}).MayRead(localPeer, "Org B", &none) {
			t.Fatalf("privileged role %s denied", role)
		}
	}
	for _, role := range []userrole.UserRole{userrole.ContractObserver, userrole.ContractSigner, userrole.ContractManager, userrole.ComplianceOfficer, userrole.ArchiveManager} {
		if (Scope{Organization: "Org A", Roles: userrole.UserRoles{role}, LocalPeer: localPeer}).MayRead(localPeer, "Org B", &none) {
			t.Fatalf("role %s reads a foreign contract", role)
		}
	}
}

func TestPredicateDecision(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want rowFilter
	}{
		{"no scope", ctx, noRows},
		{"user scope", WithScope(ctx, Scope{Organization: "Org A", Roles: userrole.UserRoles{userrole.ContractObserver}, LocalPeer: localPeer}), partyRows},
		{"privileged role", WithScope(ctx, Scope{Roles: userrole.UserRoles{userrole.Auditor}, LocalPeer: localPeer}), allRows},
		{"system scope", WithSystemScope(ctx), allRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := decide(tc.ctx); got != tc.want {
				t.Fatalf("decide = %v, want %v", got, tc.want)
			}
		})
	}
}
