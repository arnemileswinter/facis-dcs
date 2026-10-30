package auth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	goa "goa.design/goa/v3/pkg"
	"goa.design/goa/v3/security"

	"digital-contracting-service/internal/auth/db"
	"digital-contracting-service/internal/middleware"
)

// nopConn is a database whose transactions do nothing; the repositories below
// are stubs that ignore the transaction handle.
type nopConn struct{}

func (nopConn) Connect(context.Context) (driver.Conn, error) { return nopConn{}, nil }
func (nopConn) Driver() driver.Driver                        { return nil }
func (nopConn) Prepare(string) (driver.Stmt, error)          { return nil, errors.New("no statements") }
func (nopConn) Close() error                                 { return nil }
func (nopConn) Begin() (driver.Tx, error)                    { return nopConn{}, nil }
func (nopConn) Commit() error                                { return nil }
func (nopConn) Rollback() error                              { return nil }

type stubAttempts struct{ failed int }

func (s *stubAttempts) Create(_ context.Context, _ *sqlx.Tx, a db.AccessAttempt) error {
	if !a.Success {
		s.failed++
	}
	return nil
}
func (s *stubAttempts) ReadByUserID(context.Context, *sqlx.Tx, string) ([]db.AccessAttempt, error) {
	return nil, nil
}
func (s *stubAttempts) ReadByIP(context.Context, *sqlx.Tx, string) ([]db.AccessAttempt, error) {
	return nil, nil
}
func (s *stubAttempts) CountFailedAttempts(context.Context, *sqlx.Tx, string, time.Time) (int, error) {
	return s.failed, nil
}
func (s *stubAttempts) CountFailedAttemptsByIP(context.Context, *sqlx.Tx, string, time.Time) (int, error) {
	return s.failed, nil
}
func (s *stubAttempts) DeleteOlderThan(context.Context, *sqlx.Tx, time.Time) error { return nil }
func (s *stubAttempts) ReadPage(context.Context, *sqlx.Tx, *db.AccessAttemptCursor, int) ([]db.AccessAttempt, error) {
	return nil, nil
}

type stubLocks struct{ locked bool }

func (s *stubLocks) IsLocked(context.Context, *sqlx.Tx, string) (bool, *time.Time, error) {
	return s.locked, nil, nil
}
func (s *stubLocks) SetLockout(context.Context, *sqlx.Tx, string, time.Time) error {
	s.locked = true
	return nil
}
func (s *stubLocks) ClearLockout(context.Context, *sqlx.Tx, string) error {
	s.locked = false
	return nil
}

func unsignedToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return enc(map[string]string{"alg": "none"}) + "." + enc(claims) + "."
}

// FR-UC-01-4: once an IP is locked out, a valid credential from it is refused
// with the prescribed message until the lock ends.
func TestLockoutRefusesAValidCredential(t *testing.T) {
	// The signature check is not under test here; TestValidateTokenRefusesForgedTokens covers it.
	t.Setenv("JWT_ALG_NONE_SUPPORTED", "true")
	var issuer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "jwks_uri": issuer + "/jwks"})
	}))
	t.Cleanup(srv.Close)
	issuer = srv.URL

	validator, err := middleware.NewHydraJWTValidator(context.Background(), middleware.HydraJWTConfig{
		PublicIssuerURL: issuer,
		ClientID:        "dcs-client",
	})
	if err != nil {
		t.Fatalf("discovery failed: %v", err)
	}
	locks := &stubLocks{}
	authenticator := JWTAuthenticator{
		Validator:    validator,
		DB:           sqlx.NewDb(sql.OpenDB(nopConn{}), "postgres"),
		AAttemptRepo: &stubAttempts{},
		LockRepo:     locks,
	}

	ctx := context.WithValue(context.Background(), middleware.IPKey, "203.0.113.7")
	ctx = context.WithValue(ctx, goa.ServiceKey, "template_repository")
	ctx = context.WithValue(ctx, goa.MethodKey, "search")
	scheme := &security.JWTScheme{RequiredScopes: []string{"Template Creator"}}
	accessToken := func(exp time.Time) string {
		return unsignedToken(t, map[string]any{
			"iss":       issuer,
			"sub":       "did:example:holder",
			"client_id": "dcs-client",
			"exp":       exp.Unix(),
			"ext":       map[string]any{"iss": "did:web:participant.example", "roles": []string{"Template Creator"}},
		})
	}
	valid := accessToken(time.Now().Add(time.Hour))

	// A burst of requests that all carry the same just-expired token is how
	// every client meets the end of a token's lifetime. It is refused, but it
	// does not lock the client out.
	expired := accessToken(time.Now().Add(-time.Minute))
	for range 6 {
		if _, err := authenticator.JWTAuth(ctx, expired, scheme); err == nil || err.Error() != CredentialRefused {
			t.Fatalf("an expired credential got %v, not %q", err, CredentialRefused)
		}
	}
	if locks.locked {
		t.Fatal("expired access tokens locked the client out")
	}
	if _, err := authenticator.JWTAuth(ctx, valid, scheme); err != nil {
		t.Fatalf("a valid credential was refused before any lockout: %v", err)
	}

	for i := 0; i < 5 && !locks.locked; i++ {
		if _, err := authenticator.JWTAuth(ctx, "not-a-token", scheme); err == nil || err.Error() != CredentialRefused {
			t.Fatalf("a malformed credential got %v, not %q", err, CredentialRefused)
		}
	}
	if !locks.locked {
		t.Fatal("repeated invalid credentials did not lock the IP out")
	}

	_, err = authenticator.JWTAuth(ctx, valid, scheme)
	if err == nil {
		t.Fatal("a valid credential got through during the lockout")
	}
	if err.Error() != CredentialRefused {
		t.Fatalf("lockout refusal reads %q, not %q", err.Error(), CredentialRefused)
	}
	if !locks.locked {
		t.Fatal("a valid credential cleared the lockout")
	}
}
