package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	goa "goa.design/goa/v3/pkg"

	genauth "digital-contracting-service/gen/auth"
	"digital-contracting-service/internal/auth"
	authdb "digital-contracting-service/internal/auth/db"
	pg "digital-contracting-service/internal/auth/db/pq"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/pgtest"
	"digital-contracting-service/internal/middleware"
)

// holderPresentation returns a vp_token whose credential binds a fresh holder
// key, issued by an issuer this instance does not trust, and that key as
// did:jwk.
func holderPresentation(t *testing.T) (string, string) {
	t.Helper()
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	pub, err := holderKey.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	x, y := b64(pub[1:33]), b64(pub[33:])
	issuerJWT, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": "did:web:untrusted.example",
		"vct": "dcs-poa",
		"cnf": map[string]any{"jwk": map[string]any{"kty": "EC", "crv": "P-256", "x": x, "y": y}},
	}).SignedString(issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	vp, err := json.Marshal(map[string][]string{"q": {issuerJWT + "~e30.e30.e30"}})
	if err != nil {
		t.Fatal(err)
	}
	did, _ := json.Marshal(map[string]string{"crv": "P-256", "kty": "EC", "x": x, "y": y})
	return string(vp), "did:jwk:" + b64(did)
}

// UC-IF-2, UC-IF-3 (FR-UC-01-4): a refused login presentation gets the
// prescribed message, also as the status the login page shows, and repeated
// refused presentations lock out the holder key they name, not the address
// the wallet posts from.
func TestRepeatedInvalidPresentationsLockTheHolder(t *testing.T) {
	database := pgtest.Open(t)
	presentations := pg.NewPostgresPresentationAttemptRepo(database)
	attempts := &pg.PostgresAccessAttemptRepo{}
	svc := &authSvc{
		JWTAuthenticator: auth.NewJWTAuthenticator(nil, database, attempts, &pg.PostgresLockoutRepo{}),
		presentations:    presentations,
		dcqlQuery:        map[string]any{"credentials": []any{map[string]any{"id": "q"}}},
	}
	const walletIP = "198.51.100.20"
	ctx := context.WithValue(context.Background(), middleware.IPKey, walletIP)
	ctx = context.WithValue(ctx, goa.ServiceKey, genauth.ServiceName)
	ctx = context.WithValue(ctx, goa.MethodKey, "presentationCallback")

	present := func(vp string) (*authdb.PresentationAttempt, error) {
		t.Helper()
		state := rand.Text()
		if err := presentations.CreatePending(ctx, authdb.PresentationAttempt{PresentationState: state, Nonce: "n", ExpiresAt: time.Now().UTC().Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		if err := presentations.SetHydraLoginChallenge(ctx, state, "challenge"); err != nil {
			t.Fatal(err)
		}
		_, err := svc.PresentationCallback(ctx, &genauth.PresentationCallbackPayload{State: state, VpToken: &vp})
		attempt, findErr := presentations.FindByPresentationState(ctx, state)
		if findErr != nil {
			t.Fatal(findErr)
		}
		return attempt, err
	}
	refused := func(attempt *authdb.PresentationAttempt, err error) {
		t.Helper()
		var named interface{ ErrorName() string }
		if !errors.As(err, &named) || named.ErrorName() != "unauthorized" || err.Error() != auth.CredentialRefused {
			t.Fatalf("refusal = %v, want unauthorized %q", err, auth.CredentialRefused)
		}
		if attempt.ErrorMessage == nil || *attempt.ErrorMessage != auth.CredentialRefused {
			t.Fatalf("status message = %v, want %q", attempt.ErrorMessage, auth.CredentialRefused)
		}
	}
	countFor := func(key string) int {
		t.Helper()
		var n int
		if err := database.Get(&n, `SELECT count(*) FROM access_attempts WHERE lockout_key = $1`, key); err != nil {
			t.Fatal(err)
		}
		return n
	}

	vp, did := holderPresentation(t)
	holderKey := "holder:" + did
	threshold := conf.LoginAttemptsThresholdInDuration()
	for i := 1; i <= threshold; i++ {
		refused(present(vp))
		if _, locked := svc.LockedUntil(ctx, holderKey); locked != (i == threshold) {
			t.Fatalf("after %d refused presentations the holder lock is %t", i, locked)
		}
	}
	if _, locked := svc.LockedUntil(ctx, walletIP); locked {
		t.Fatal("the wallet address was locked instead of the holder")
	}

	// A locked holder is refused before verification, and the refusal does
	// not count again.
	refused(present(vp))
	if n := countFor(holderKey); n != threshold {
		t.Fatalf("counted attempts for the locked holder = %d, want %d", n, threshold)
	}

	// Another holder posting from the same wallet address is not locked.
	other, otherDID := holderPresentation(t)
	refused(present(other))
	if n := countFor("holder:" + otherDID); n != 1 {
		t.Fatalf("the other holder's refusal counted %d times, want 1", n)
	}
}

// NFR-18: the administrator unlock ends an active lock, is audited, and
// answers not_found when the key has no active lock.
func TestUnlockLiftsAnActiveLockout(t *testing.T) {
	database := pgtest.Open(t)
	locks := &pg.PostgresLockoutRepo{}
	svc := &authSvc{JWTAuthenticator: auth.NewJWTAuthenticator(nil, database, &pg.PostgresAccessAttemptRepo{}, locks)}
	ctx := middleware.InjectAuthContext(context.Background(), []string{"Sys. Administrator"}, "did:example:admin", "did:web:operator.example")
	if _, err := database.Exec(`INSERT INTO lockouts (lockout_key, locked_until) VALUES ('203.0.113.9', now() + interval '10 minutes')`); err != nil {
		t.Fatal(err)
	}

	listed, err := svc.Lockouts(ctx, &genauth.LockoutsPayload{})
	if err != nil || len(listed) != 1 || listed[0].Key != "203.0.113.9" {
		t.Fatalf("lockouts = %+v, %v", listed, err)
	}
	if err := svc.Unlock(ctx, &genauth.UnlockPayload{Key: "203.0.113.9"}); err != nil {
		t.Fatal(err)
	}
	if _, locked := svc.LockedUntil(ctx, "203.0.113.9"); locked {
		t.Fatal("the lock is still active after the unlock")
	}
	var events int
	if err := database.Get(&events, `SELECT count(*) FROM outbox_events WHERE event_type = 'LOCKOUT_LIFTED'`); err != nil || events != 1 {
		t.Fatalf("unlock audit events = %d, %v", events, err)
	}
	var named interface{ ErrorName() string }
	if err := svc.Unlock(ctx, &genauth.UnlockPayload{Key: "203.0.113.9"}); !errors.As(err, &named) || named.ErrorName() != "not_found" {
		t.Fatalf("second unlock = %v, want not_found", err)
	}
}
