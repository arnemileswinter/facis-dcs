package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"goa.design/goa/v3/security"

	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
)

// requiredScopes returns the roles the generated endpoint admits: the
// security scheme it hands to the JWT check before the service runs.
func requiredScopes(t *testing.T, newEndpoint func(security.AuthJWTFunc) func(context.Context, any) (any, error), payload any) []string {
	t.Helper()
	var scopes []string
	stop := errors.New("stop after the scope check")
	endpoint := newEndpoint(func(ctx context.Context, _ string, scheme *security.JWTScheme) (context.Context, error) {
		scopes = scheme.RequiredScopes
		return ctx, stop
	})
	_, err := endpoint(context.Background(), payload)
	require.ErrorIs(t, err, stop)
	return scopes
}

// DCS-IR-CSA-03: the Archive Manager terminates archived contracts.
func TestArchiveManagerMayTerminateAContract(t *testing.T) {
	scopes := requiredScopes(t, func(auth security.AuthJWTFunc) func(context.Context, any) (any, error) {
		return contractworkflowengine.NewTerminateEndpoint(nil, auth)
	}, &contractworkflowengine.ContractTerminateRequest{})
	require.Contains(t, scopes, "Archive Manager")
}

// DCS-FR-CSA-24: the Auditor reads the contract content from the audit viewer.
func TestAuditorMayReadAContract(t *testing.T) {
	scopes := requiredScopes(t, func(auth security.AuthJWTFunc) func(context.Context, any) (any, error) {
		return contractworkflowengine.NewRetrieveByIDEndpoint(nil, auth)
	}, &contractworkflowengine.ContractRetrieveByIDRequest{})
	require.Contains(t, scopes, "Auditor")
}
