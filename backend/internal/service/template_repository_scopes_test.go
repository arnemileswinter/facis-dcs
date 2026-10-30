package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"goa.design/goa/v3/security"

	templaterepository "digital-contracting-service/gen/template_repository"
)

// templateScopeRecorder stops every endpoint at its JWT check and keeps the scopes it requires.
type templateScopeRecorder struct {
	templaterepository.Service
	required []string
}

func (r *templateScopeRecorder) JWTAuth(ctx context.Context, _ string, scheme *security.JWTScheme) (context.Context, error) {
	r.required = scheme.RequiredScopes
	return ctx, errors.New("stop")
}

func requiredTemplateScopes(t *testing.T, call func(*templaterepository.Endpoints) error) []string {
	t.Helper()
	recorder := &templateScopeRecorder{}
	if err := call(templaterepository.NewEndpoints(recorder)); err == nil {
		t.Fatal("endpoint passed the JWT check")
	}
	return recorder.required
}

// DCS-FR-TR-06: the Template Manager does not approve or reject; DCS-IR-TR-07: it reads the template audit.
func TestTemplateRepositoryScopes(t *testing.T) {
	ctx := context.Background()
	approve := requiredTemplateScopes(t, func(e *templaterepository.Endpoints) error {
		_, err := e.Approve(ctx, &templaterepository.ContractTemplateApproveRequest{})
		return err
	})
	reject := requiredTemplateScopes(t, func(e *templaterepository.Endpoints) error {
		_, err := e.Reject(ctx, &templaterepository.ContractTemplateRejectRequest{})
		return err
	})
	audit := requiredTemplateScopes(t, func(e *templaterepository.Endpoints) error {
		_, err := e.Audit(ctx, &templaterepository.ContractTemplateAuditRequest{})
		return err
	})
	for name, scopes := range map[string][]string{"approve": approve, "reject": reject} {
		if !slices.Equal(scopes, []string{"Template Approver"}) {
			t.Errorf("%s scopes = %v, want only Template Approver", name, scopes)
		}
	}
	if !slices.Contains(audit, "Template Manager") {
		t.Errorf("audit scopes = %v, want Template Manager", audit)
	}
}
