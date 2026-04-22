package service

import (
	"context"
	"digital-contracting-service/internal/pathutil"
	"net/http"
)

const (
	refreshTokenCookieName = "refresh_token"
	oauthStateCookieName   = "oidc_state"
	idTokenCookieName      = "id_token"
)

const apiPathPrefixEnv = "DCS_API_PATH"
const defaultAPIPathPrefix = ""

// contextKey is a private type for context keys in this package.
type contextKey int

const (
	httpRequestKey    contextKey = iota
	refreshTokenKey   contextKey = iota
	responseWriterKey contextKey = iota
)

// RequestContextMiddleware injects the *http.Request and http.ResponseWriter
// into the context so that service implementations can access them.
// This is used by the Auth service to read cookies and set cookie headers.
func RequestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), httpRequestKey, r)
		ctx = context.WithValue(ctx, responseWriterKey, w)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HTTPRequestFromContext extracts the *http.Request from context.
func HTTPRequestFromContext(ctx context.Context) (*http.Request, bool) {
	r, ok := ctx.Value(httpRequestKey).(*http.Request)
	return r, ok
}

// ResponseWriterFromContext extracts the http.ResponseWriter from context.
func ResponseWriterFromContext(ctx context.Context) (http.ResponseWriter, bool) {
	w, ok := ctx.Value(responseWriterKey).(http.ResponseWriter)
	return w, ok
}

// SetRefreshTokenInContext stores the refresh token in context for the
// response encoder to pick up and set as a cookie.
// Since context is immutable, this uses the ResponseWriter to set the cookie
// header directly.
func SetRefreshTokenInContext(ctx context.Context, refreshToken string) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok || refreshToken == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     refreshTokenCookiePath(),
		MaxAge:   7 * 24 * 60 * 60, // 7 days
	})
}

// ClearRefreshTokenCookie clears the refresh token cookie by setting MaxAge to -1.
func ClearRefreshTokenCookie(ctx context.Context) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     refreshTokenCookiePath(),
		MaxAge:   -1,
	})
}

// SetOAuthStateCookie stores the transient OIDC state value used by the callback.
func SetOAuthStateCookie(ctx context.Context, state string) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok || state == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     oauthStateCookiePath(),
		MaxAge:   10 * 60,
	})
}

// ReadOAuthStateCookie extracts the transient OIDC state cookie from the request.
func ReadOAuthStateCookie(ctx context.Context) (string, error) {
	r, ok := HTTPRequestFromContext(ctx)
	if !ok {
		return "", http.ErrNoCookie
	}
	cookie, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// ClearOAuthStateCookie clears the transient OIDC state cookie.
func ClearOAuthStateCookie(ctx context.Context) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     oauthStateCookiePath(),
		MaxAge:   -1,
	})
}

func refreshTokenCookiePath() string {
	return pathutil.JoinPaths(apiPathPrefixEnv, defaultAPIPathPrefix, "/auth/refresh")
}

func oauthStateCookiePath() string {
	return pathutil.JoinPaths(apiPathPrefixEnv, defaultAPIPathPrefix, "/auth/callback")
}

func idTokenCookiePath() string {
	return pathutil.JoinPaths(apiPathPrefixEnv, defaultAPIPathPrefix, "/auth/logout")
}

// SetIDTokenCookie persists the OIDC id_token so /auth/logout can pass
// it to the provider as id_token_hint.
func SetIDTokenCookie(ctx context.Context, idToken string) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok || idToken == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     idTokenCookieName,
		Value:    idToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     idTokenCookiePath(),
		MaxAge:   7 * 24 * 60 * 60,
	})
}

// ReadIDTokenCookie retrieves the stored id_token if present.
func ReadIDTokenCookie(ctx context.Context) string {
	r, ok := HTTPRequestFromContext(ctx)
	if !ok {
		return ""
	}
	cookie, err := r.Cookie(idTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ClearIDTokenCookie expires the id_token cookie.
func ClearIDTokenCookie(ctx context.Context) {
	w, ok := ResponseWriterFromContext(ctx)
	if !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     idTokenCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     idTokenCookiePath(),
		MaxAge:   -1,
	})
}
