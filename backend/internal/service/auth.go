package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	genauth "digital-contracting-service/gen/auth"
	"digital-contracting-service/internal/pathutil"

	"goa.design/clue/log"
	goa "goa.design/goa/v3/pkg"
)

// authSvc implements the generated auth.Service interface.
type authSvc struct {
	oidcIssuerURL     string
	oidcClientID      string
	redirectURI       string
	logoutRedirectURI string
	uiBasePath        string
	metadataMu        sync.RWMutex
	metadata          *oidcProviderMetadata
}

type oidcProviderMetadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// NewAuth returns the Auth service implementation.
func NewAuth() genauth.Service {
	return &authSvc{
		oidcIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
		oidcClientID:      os.Getenv("OIDC_CLIENT_ID"),
		redirectURI:       os.Getenv("OIDC_REDIRECT_URI"),
		logoutRedirectURI: os.Getenv("OIDC_LOGOUT_REDIRECT_URI"),
		uiBasePath:        pathutil.NormalizePath(os.Getenv("DCS_UI_PATH"), "/ui/", true),
	}
}

// Login returns the OIDC authorization URL.
func (s *authSvc) Login(ctx context.Context) (*genauth.LoginResult, error) {
	log.Printf(ctx, "auth.login")
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}
	params := url.Values{}
	params.Set("client_id", s.oidcClientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	authURL := metadata.AuthorizationEndpoint + "?" + params.Encode()
	return &genauth.LoginResult{AuthURL: authURL}, nil
}

// oidcTokenResponse is the raw response from the provider token endpoint.
type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// Callback exchanges the authorization code for tokens.
// The refresh_token cookie is set by SetRefreshTokenInContext.
// After setting the cookie, it redirects to /auth/success.
func (s *authSvc) Callback(ctx context.Context, p *genauth.CallbackPayload) (*genauth.CallbackResult, error) {
	log.Printf(ctx, "auth.callback")

	tokenResp, err := s.exchangeCodeForToken(ctx, p.Code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Stash the refresh token in context so the response encoder can set the cookie.
	// This is picked up by SetRefreshTokenInContext which sets the cookie immediately.
	SetRefreshTokenInContext(ctx, tokenResp.RefreshToken)

	// Redirect to frontend auth success route under configured UI base path.
	return &genauth.CallbackResult{
		Location: s.uiBasePath + "auth/success",
	}, nil
}

// Refresh exchanges the refresh_token (from HttpOnly cookie) for a new access token.
func (s *authSvc) Refresh(ctx context.Context) (*genauth.RefreshResult, error) {
	log.Printf(ctx, "auth.refresh")

	// Extract *http.Request from context (injected by RequestContextMiddleware).
	r, ok := HTTPRequestFromContext(ctx)
	if !ok {
		return nil, goa.PermanentError("unauthorized", "missing HTTP request in context")
	}

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return nil, goa.PermanentError("unauthorized", "missing or invalid refresh token")
	}

	tokenResp, err := s.refreshAccessToken(ctx, cookie.Value)
	if err != nil {
		return nil, goa.PermanentError("unauthorized", "token refresh failed: %v", err)
	}

	return &genauth.RefreshResult{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		ExpiresIn:   tokenResp.ExpiresIn,
	}, nil
}

// Logout returns the OIDC logout URL when the provider exposes one.
func (s *authSvc) Logout(ctx context.Context) (*genauth.LogoutResult, error) {
	log.Printf(ctx, "auth.logout")
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}

	// Build provider logout URL with configured post-logout redirect.
	postLogoutRedirect := s.uiBasePath
	if s.logoutRedirectURI != "" {
		postLogoutRedirect = s.logoutRedirectURI
	}
	if metadata.EndSessionEndpoint == "" {
		return &genauth.LogoutResult{LogoutURL: postLogoutRedirect}, nil
	}

	params := url.Values{}
	params.Set("client_id", s.oidcClientID)
	params.Set("post_logout_redirect_uri", postLogoutRedirect)
	logoutURL := metadata.EndSessionEndpoint + "?" + params.Encode()

	return &genauth.LogoutResult{
		LogoutURL: logoutURL,
	}, nil
}

// exchangeCodeForToken POSTs the auth code to the provider token endpoint.
func (s *authSvc) exchangeCodeForToken(ctx context.Context, code string) (*oidcTokenResponse, error) {
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}
	tokenEndpoint := metadata.TokenEndpoint
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", s.oidcClientID)
	data.Set("redirect_uri", s.redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp oidcTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	return &tokenResp, nil
}

// refreshAccessToken asks the provider for a new access token using the refresh token.
func (s *authSvc) refreshAccessToken(ctx context.Context, refreshToken string) (*oidcTokenResponse, error) {
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}
	tokenEndpoint := metadata.TokenEndpoint
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", s.oidcClientID)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp oidcTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	return &tokenResp, nil
}

// revokeToken revokes a refresh token when the provider exposes a revocation endpoint.
func (s *authSvc) revokeToken(ctx context.Context, refreshToken string) error {
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return fmt.Errorf("oidc discovery failed: %w", err)
	}
	if metadata.RevocationEndpoint == "" {
		return nil
	}
	revokeEndpoint := metadata.RevocationEndpoint
	data := url.Values{}
	data.Set("token", refreshToken)
	data.Set("client_id", s.oidcClientID)
	data.Set("token_type_hint", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", revokeEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke failed with status %d", resp.StatusCode)
	}
	return nil
}

// LogoutComplete finalizes logout by revoking the refresh token and clearing the cookie.
// This endpoint is called after the provider logout flow completes.
func (s *authSvc) LogoutComplete(ctx context.Context) (*genauth.LogoutCompleteResult, error) {
	log.Printf(ctx, "auth.logout-complete")

	// Extract *http.Request from context
	r, ok := HTTPRequestFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing HTTP request in context")
	}

	// Try to get and revoke the refresh token
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" {
		// Best effort: revoke the token with the OIDC provider.
		_ = s.revokeToken(ctx, cookie.Value)
	}

	// Clear the refresh token cookie
	ClearRefreshTokenCookie(ctx)

	// Redirect to frontend UI under configured base path
	return &genauth.LogoutCompleteResult{
		Location: s.uiBasePath,
	}, nil
}

func (s *authSvc) providerMetadata(ctx context.Context) (*oidcProviderMetadata, error) {
	s.metadataMu.RLock()
	if s.metadata != nil {
		metadata := s.metadata
		s.metadataMu.RUnlock()
		return metadata, nil
	}
	s.metadataMu.RUnlock()

	issuer := strings.TrimRight(s.oidcIssuerURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openid-configuration returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var metadata oidcProviderMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}
	if metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("openid-configuration missing required endpoints")
	}

	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if s.metadata == nil {
		s.metadata = &metadata
	}
	return s.metadata, nil
}
