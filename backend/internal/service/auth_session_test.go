package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"digital-contracting-service/internal/auth"
	"digital-contracting-service/internal/auth/hydra"
)

// refreshingHydra answers the token endpoint with an access token whose
// session names a PoA status reference, and records every revocation.
func refreshingHydra(t *testing.T) (*hydra.Client, *[]string) {
	t.Helper()
	revoked := &[]string{}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": srv.URL + "/oauth2/auth",
			"token_endpoint":         srv.URL + "/oauth2/token",
			"revocation_endpoint":    srv.URL + "/oauth2/revoke",
		})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		payload, _ := json.Marshal(map[string]any{
			"sub": "did:example:holder",
			"ext": map[string]any{
				"roles": []string{"Template Creator"},
				hydra.CredentialStatusClaim: map[string]any{
					"iss":    "https://issuer.example",
					"status": map[string]any{"status_list": map[string]any{"idx": 7, "uri": "https://issuer.example/status/1"}},
				},
			},
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(payload) + ".c2ln",
			"refresh_token": "rotated-refresh-token",
			"token_type":    "bearer",
			"expires_in":    300,
		})
	})
	mux.HandleFunc("/oauth2/revoke", func(w http.ResponseWriter, r *http.Request) {
		*revoked = append(*revoked, r.FormValue("token"))
	})

	return hydra.New(hydra.Config{PublicIssuerURL: srv.URL, ClientID: "dcs-client", AdminURL: srv.URL}), revoked
}

func refreshRequest() (context.Context, *httptest.ResponseRecorder) {
	r := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-refresh-token"})
	w := httptest.NewRecorder()
	ctx := context.WithValue(context.Background(), httpRequestKey, r)
	return context.WithValue(ctx, responseWriterKey, http.ResponseWriter(w)), w
}

func refreshCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

// NFR-BR-06, IR-CI-09: refresh checks the status of the PoA the session was
// opened with. A revoked PoA ends the session; an active one keeps it.
func TestRefreshEndsTheSessionOfARevokedCredential(t *testing.T) {
	client, revoked := refreshingHydra(t)
	var checked json.RawMessage
	revocation := error(nil)
	svc := &authSvc{hydra: client, checkCredentialStatus: func(ref json.RawMessage) error {
		checked = ref
		return revocation
	}}

	ctx, w := refreshRequest()
	res, err := svc.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh with an active credential failed: %v", err)
	}
	if res.AccessToken == "" {
		t.Fatal("no access token handed out")
	}
	var ref map[string]any
	if err := json.Unmarshal(checked, &ref); err != nil || ref["iss"] != "https://issuer.example" || ref["status"] == nil {
		t.Fatalf("the status check did not get the session's reference: %s (%v)", checked, err)
	}
	if c := refreshCookie(w); c == nil || c.Value != "rotated-refresh-token" {
		t.Fatalf("the rotated refresh token was not handed out: %+v", c)
	}

	revocation = errors.New("credential status list index 7 is revoked")
	ctx, w = refreshRequest()
	res, err = svc.Refresh(ctx)
	if err == nil {
		t.Fatalf("refresh with a revoked credential handed out %+v", res)
	}
	if err.Error() != auth.CredentialRefused {
		t.Fatalf("refusal reads %q, not %q", err.Error(), auth.CredentialRefused)
	}
	if len(*revoked) != 1 || (*revoked)[0] != "rotated-refresh-token" {
		t.Fatalf("the session's refresh token was not revoked at Hydra: %v", *revoked)
	}
	if c := refreshCookie(w); c == nil || c.MaxAge >= 0 {
		t.Fatalf("the refresh cookie was not cleared: %+v", c)
	}
}

// A session whose token names no credential cannot be checked, so it is refused.
func TestRefreshRefusesASessionWithoutAStatusReference(t *testing.T) {
	svc := &authSvc{checkCredentialStatus: func(json.RawMessage) error { return nil }}
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"did:example:holder","ext":{"roles":["Template Creator"]}}`))
	if err := svc.checkSessionCredential("eyJhbGciOiJSUzI1NiJ9." + payload + ".c2ln"); err == nil {
		t.Fatal("a session without a credential status reference was accepted")
	}
}
