package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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

const oauthStateSizeBytes = 24

// authSvc implements the generated auth.Service interface.
type authSvc struct {
	oidcIssuerURL          string
	oidcClientID           string
	oidcClientSecret       string
	redirectURI            string
	logoutRedirectURI      string
	uiBasePath             string
	cvURL                  string
	presentationGroup      string
	presentationNamespace  string
	presentationKey        string
	presentationDid        string
	presentationPublicHost string
	metadataMu             sync.RWMutex
	metadata               *oidcProviderMetadata
	presentationMu         sync.RWMutex
	presentationState      map[string]*presentationState
}

type presentationState struct {
	Completed bool
	Roles     []string
	Subject   string
	Challenge string
	Location  string
	Nonce     string
	ClientID  string
}

type hydraAcceptResp struct {
	RedirectTo string `json:"redirect_to"`
}

type hydraLoginAcceptReq struct {
	Subject     string         `json:"subject"`
	Remember    bool           `json:"remember"`
	RememberFor int            `json:"remember_for"`
	Context     map[string]any `json:"context,omitempty"`
}

type hydraConsentRequest struct {
	RequestedScope               []string       `json:"requested_scope"`
	RequestedAccessTokenAudience []string       `json:"requested_access_token_audience"`
	Context                      map[string]any `json:"context"`
}

type hydraConsentAcceptReq struct {
	GrantScope               []string `json:"grant_scope"`
	GrantAccessTokenAudience []string `json:"grant_access_token_audience"`
	Remember                 bool     `json:"remember"`
	RememberFor              int      `json:"remember_for"`
	Session                  struct {
		AccessToken map[string]any `json:"access_token,omitempty"`
		IDToken     map[string]any `json:"id_token,omitempty"`
	} `json:"session,omitempty"`
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
		oidcIssuerURL:         os.Getenv("OIDC_ISSUER_URL"),
		oidcClientID:          os.Getenv("OIDC_CLIENT_ID"),
		oidcClientSecret:      os.Getenv("OIDC_CLIENT_SECRET"),
		redirectURI:           os.Getenv("OIDC_REDIRECT_URI"),
		logoutRedirectURI:     os.Getenv("OIDC_LOGOUT_REDIRECT_URI"),
		uiBasePath:            pathutil.NormalizePath(os.Getenv("DCS_UI_PATH"), "/ui/", true),
		cvURL:                 strings.TrimRight(strings.TrimSpace(os.Getenv("CREDENTIAL_VERIFICATION_URL")), "/"),
		presentationGroup:     strings.TrimSpace(os.Getenv("OCM_W_PRESENTATION_GROUP")),
		presentationNamespace: firstNonEmpty(strings.TrimSpace(os.Getenv("OCM_W_SIGNER_NAMESPACE")), "tenant_space"),
		presentationKey:       firstNonEmpty(strings.TrimSpace(os.Getenv("OCM_W_SIGNER_KEY")), "signerkey"),
		presentationDid:       strings.TrimSpace(os.Getenv("OCM_W_PRESENTATION_DID")),
		// Host advertised to the wallet inside the OID4VP request_object's response_uri.
		// CV builds response_uri from the incoming HTTP Host header, so we override it
		// with the public gateway host (e.g. localhost:5173 in dev, ingress in prod).
		presentationPublicHost: strings.TrimSpace(os.Getenv("PRESENTATION_PUBLIC_HOST")),
		presentationState:      map[string]*presentationState{},
	}
}

// Login returns the OIDC authorization URL.
func (s *authSvc) Login(ctx context.Context) (*genauth.LoginResult, error) {
	log.Printf(ctx, "auth.login")
	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}
	state, err := newOAuthState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate oidc state: %w", err)
	}
	SetOAuthStateCookie(ctx, state)

	params := url.Values{}
	params.Set("client_id", s.oidcClientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("response_type", "code")
	// offline_access is required for Hydra to issue a refresh_token. Without
	// it the token endpoint returns access_token only and the cookie-backed
	// /auth/refresh flow never works.
	params.Set("scope", "openid offline_access")
	params.Set("state", state)
	authURL := metadata.AuthorizationEndpoint + "?" + params.Encode()
	return &genauth.LoginResult{AuthURL: authURL}, nil
}

// oidcTokenResponse is the raw response from the provider token endpoint.
type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
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
	defer ClearOAuthStateCookie(ctx)

	returnedState := ""
	if p.State != nil {
		returnedState = *p.State
	}
	if err := s.validateOAuthState(ctx, returnedState); err != nil {
		return nil, goa.PermanentError("unauthorized", "invalid oidc state: %v", err)
	}

	tokenResp, err := s.exchangeCodeForToken(ctx, p.Code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Stash the refresh token in context so the response encoder can set the cookie.
	// This is picked up by SetRefreshTokenInContext which sets the cookie immediately.
	SetRefreshTokenInContext(ctx, tokenResp.RefreshToken)
	SetIDTokenCookie(ctx, tokenResp.IDToken)

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
		// Clear the stale cookie so the frontend stops retrying with a dead token
		// and falls back to a fresh login.
		ClearRefreshTokenCookie(ctx)
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

	// Capture the stored id_token before we drop the cookie; Hydra needs it
	// as id_token_hint to honour post_logout_redirect_uri.
	idToken := ReadIDTokenCookie(ctx)

	// Drop our session cookies so the next bootstrap call starts fresh.
	ClearRefreshTokenCookie(ctx)
	ClearIDTokenCookie(ctx)

	postLogoutRedirect := s.uiBasePath
	if s.logoutRedirectURI != "" {
		postLogoutRedirect = s.logoutRedirectURI
	}

	metadata, err := s.providerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed: %w", err)
	}
	if metadata.EndSessionEndpoint == "" {
		return &genauth.LogoutResult{LogoutURL: postLogoutRedirect}, nil
	}

	params := url.Values{}
	params.Set("client_id", s.oidcClientID)
	if idToken != "" {
		params.Set("id_token_hint", idToken)
		params.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	logoutURL := metadata.EndSessionEndpoint + "?" + params.Encode()
	return &genauth.LogoutResult{LogoutURL: logoutURL}, nil
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
	if s.oidcClientSecret != "" {
		data.Set("client_secret", s.oidcClientSecret)
	}
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
	if s.oidcClientSecret != "" {
		data.Set("client_secret", s.oidcClientSecret)
	}

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
	if s.oidcClientSecret != "" {
		data.Set("client_secret", s.oidcClientSecret)
	}
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

func (s *authSvc) validateOAuthState(ctx context.Context, returnedState string) error {
	expectedState, err := ReadOAuthStateCookie(ctx)
	if err != nil {
		return fmt.Errorf("missing stored state: %w", err)
	}
	if returnedState == "" {
		return fmt.Errorf("missing callback state")
	}
	if subtle.ConstantTimeCompare([]byte(expectedState), []byte(returnedState)) != 1 {
		return fmt.Errorf("callback state mismatch")
	}
	return nil
}

func newOAuthState() (string, error) {
	buf := make([]byte, oauthStateSizeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// PresentationRequest generates an OpenID4VP presentation request URI by delegating to the CV service.
func (s *authSvc) PresentationRequest(ctx context.Context, p *genauth.PresentationRequestPayload) (*genauth.PresentationRequestResult, error) {
	log.Printf(ctx, "auth.presentation-request")

	if s.cvURL == "" {
		return nil, goa.PermanentError("internal", "CREDENTIAL_VERIFICATION_URL not configured")
	}

	// Generate state and nonce for correlation with callback.
	requestID, err := newOAuthState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate request id: %w", err)
	}
	nonce, err := newOAuthState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Build presentation definition for role credential.
	presentationDef := map[string]any{
		"id": "dcs_role_presentation",
		"input_descriptors": []map[string]any{
			{
				"id": "dcs_role_credential",
				"constraints": map[string]any{
					"fields": []map[string]any{
						{"path": []string{"$.credentialSubject.role"}},
					},
				},
			},
		},
	}
	presentationDefJSON, _ := json.Marshal(presentationDef)
	presentationDefB64 := base64.URLEncoding.EncodeToString(presentationDefJSON)

	cvReqURL := fmt.Sprintf("%s/v1/tenants/tenant_space/presentation/request?requestId=%s&groupId=%s&ttl=300&presentationDefinition=%s",
		s.cvURL,
		url.QueryEscape(requestID),
		url.QueryEscape(s.presentationGroup),
		url.QueryEscape(presentationDefB64),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cvReqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build cv request: %w", err)
	}
	// CV builds the JWT response_uri from the incoming Host header. Force it to the
	// public gateway host so the wallet can reach the proof endpoint via the proxy.
	if s.presentationPublicHost != "" {
		req.Host = s.presentationPublicHost
	}
	// CV reads x-tenantId as signer namespace, x-key as signer key, x-did as JWT client_id.
	// See eclipse-xfsc/oid4-vci-credential-verification-service:
	//   internal/api/externalRoutes.go HandleRequestPresentation -> JwtResponse
	//   internal/services/presentationRequestor.go GetRequestObjectAndSetObjectFetched
	req.Header.Set("x-tenantId", s.presentationNamespace)
	req.Header.Set("x-ttl", "300")
	req.Header.Set("x-key", s.presentationKey)
	if s.presentationDid != "" {
		req.Header.Set("x-did", s.presentationDid)
	}
	if s.presentationGroup != "" {
		req.Header.Set("x-groupId", s.presentationGroup)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cv service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("failed to read cv response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, goa.PermanentError("internal", "cv service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	requestObject := extractRequestObjectFromCVResponse(body)
	if requestObject == "" {
		return nil, goa.PermanentError("internal", "cv service returned empty request object")
	}

	presentationURI := requestObject
	if !strings.HasPrefix(requestObject, "openid4vp://") {
		params := url.Values{}
		params.Set("request", requestObject)
		params.Set("state", requestID)
		presentationURI = "openid4vp://authorize?" + params.Encode()
	}

	// Store state for callback correlation and proof submission.
	challenge := ""
	if p != nil && p.LoginChallenge != nil {
		challenge = strings.TrimSpace(*p.LoginChallenge)
	}

	s.presentationMu.Lock()
	s.presentationState[requestID] = &presentationState{
		Completed: false,
		Challenge: challenge,
		Nonce:     nonce,
		ClientID:  "redirect_uri:" + s.redirectURI,
	}
	s.presentationMu.Unlock()

	return &genauth.PresentationRequestResult{
		PresentationURI: presentationURI,
	}, nil
}

// PresentationCallback handles VP submission from wallet and establishes authenticated session.
func (s *authSvc) PresentationCallback(ctx context.Context, p *genauth.PresentationCallbackPayload) (*genauth.PresentationCallbackResult, error) {
	r, _ := HTTPRequestFromContext(ctx)

	vpToken := strings.TrimSpace(p.VpToken)
	if vpToken == "" && r != nil {
		vpToken = strings.TrimSpace(r.FormValue("vp_token"))
	}
	log.Printf(ctx, "auth.presentation-callback vp_token=%s", truncateForLog(vpToken))

	if vpToken == "" {
		return nil, goa.PermanentError("unauthorized", "missing vp_token")
	}

	state := ""
	if p.State != nil {
		state = strings.TrimSpace(*p.State)
	}
	if state == "" {
		if p.RequestID != nil {
			state = strings.TrimSpace(*p.RequestID)
		}
	}
	if state == "" && r != nil {
		state = strings.TrimSpace(r.FormValue("state"))
	}
	if state == "" && r != nil {
		state = strings.TrimSpace(r.FormValue("request_id"))
	}
	if state == "" {
		return nil, goa.PermanentError("unauthorized", "missing state")
	}

	s.presentationMu.RLock()
	stored := s.presentationState[state]
	s.presentationMu.RUnlock()
	if stored == nil {
		return nil, goa.PermanentError("unauthorized", "unknown state")
	}

	// Parse VP to extract role and subject.
	claims, err := parseUnverifiedJWTClaims(vpToken)
	if err != nil {
		return nil, goa.PermanentError("unauthorized", "invalid vp_token: %v", err)
	}

	// Extract subject and role from VP
	subject := firstNonEmpty(
		getString(claims, "sub"),
		getString(claims, "credentialSubject.id"),
		"wallet-user",
	)

	roles := extractRolesFromClaims(claims)
	if len(roles) == 0 {
		role := firstNonEmpty(
			getString(claims, "credentialSubject.role"),
			getString(claims, "vc.credentialSubject.role"),
		)
		if role != "" {
			roles = []string{role}
		}
	}

	if len(roles) == 0 {
		return nil, goa.PermanentError("unauthorized", "credential role claim missing")
	}

	// Enforce nonce and audience checks when present to bind response to request.
	vpNonce := strings.TrimSpace(getString(claims, "nonce"))
	if vpNonce != "" && stored.Nonce != "" && subtle.ConstantTimeCompare([]byte(vpNonce), []byte(stored.Nonce)) != 1 {
		return nil, goa.PermanentError("unauthorized", "nonce mismatch")
	}
	vpAud := strings.TrimSpace(getString(claims, "aud"))
	if vpAud != "" && stored.ClientID != "" && subtle.ConstantTimeCompare([]byte(vpAud), []byte(stored.ClientID)) != 1 {
		return nil, goa.PermanentError("unauthorized", "audience mismatch")
	}

	// Submit VP to CV service for verification and proof completion.
	if err := s.submitProofToCV(ctx, state, vpToken); err != nil {
		log.Printf(ctx, "auth.presentation-callback cv proof submission failed: %v", err)
		return nil, goa.PermanentError("unauthorized", "VP verification failed: %v", err)
	}

	location := s.uiBasePath + "auth/success"

	if stored.Challenge != "" {
		hydraRedirect, err := s.acceptHydraLoginAndConsent(ctx, stored.Challenge, subject, roles)
		if err != nil {
			return nil, goa.PermanentError("unauthorized", "hydra session establishment failed: %v", err)
		}
		location = hydraRedirect
	}

	s.presentationMu.Lock()
	stored = s.presentationState[state]
	if stored == nil {
		stored = &presentationState{}
		s.presentationState[state] = stored
	}
	stored.Completed = true
	stored.Subject = subject
	stored.Roles = append([]string(nil), roles...)
	stored.Location = location
	s.presentationMu.Unlock()

	// Redirect to frontend auth success
	return &genauth.PresentationCallbackResult{
		Location: location,
	}, nil
}

// PresentationStatus reports whether a presentation request is complete.
// Frontend polling redirects browser to returned location when complete.
//
// Completion is recorded via PresentationCallback (HTTP direct_post path)
// or via CompletePresentationFromVP (driven by the storage NATS
// subscriber that taps CV's verified-presentation stream).
func (s *authSvc) PresentationStatus(ctx context.Context, p *genauth.PresentationStatusPayload) (*genauth.PresentationStatusResult, error) {
	requestID := strings.TrimSpace(p.RequestID)
	if requestID == "" {
		return nil, goa.PermanentError("bad_request", "missing request_id")
	}

	s.presentationMu.RLock()
	state := s.presentationState[requestID]
	s.presentationMu.RUnlock()

	if state != nil && state.Completed {
		loc := state.Location
		return &genauth.PresentationStatusResult{
			Completed: true,
			Location:  &loc,
		}, nil
	}

	return &genauth.PresentationStatusResult{Completed: false}, nil
}

// CompletePresentationFromVP processes a verified VP (JWT compact form or
// LDP-VP JSON bytes) for the given pending request: extracts subject and
// roles, accepts the Hydra login + consent challenge if one was associated
// with the request, and marks the presentation as completed so the
// frontend polling loop can redirect the user.
//
// It is invoked by the NATS subscriber that taps the storage-service
// "store presentation" stream published by CV after successful VP
// verification.
func CompletePresentationFromVP(ctx context.Context, svc genauth.Service, requestID string, vp []byte) error {
	s, ok := svc.(*authSvc)
	if !ok {
		return fmt.Errorf("auth service unavailable")
	}
	if requestID == "" {
		return fmt.Errorf("missing requestID")
	}

	s.presentationMu.RLock()
	stored := s.presentationState[requestID]
	s.presentationMu.RUnlock()
	if stored == nil {
		// Either an unrelated presentation or one we never initiated.
		return nil
	}
	if stored.Completed {
		return nil
	}

	claims, err := parseVPClaims(vp)
	if err != nil {
		return fmt.Errorf("parse VP: %w", err)
	}
	subject := strings.TrimSpace(extractSubjectFromVP(claims))
	if subject == "" {
		return fmt.Errorf("VP missing holder DID (iss/holder/sub)")
	}
	roles := extractRolesFromClaims(claims)
	if len(roles) == 0 {
		return fmt.Errorf("credential role claim missing")
	}

	location := s.uiBasePath + "auth/success"
	if stored.Challenge != "" {
		hydraRedirect, err := s.acceptHydraLoginAndConsent(ctx, stored.Challenge, subject, roles)
		if err != nil {
			return fmt.Errorf("hydra session establishment: %w", err)
		}
		location = hydraRedirect
	}

	s.presentationMu.Lock()
	stored = s.presentationState[requestID]
	if stored == nil {
		stored = &presentationState{}
		s.presentationState[requestID] = stored
	}
	stored.Completed = true
	stored.Subject = subject
	stored.Roles = append([]string(nil), roles...)
	stored.Location = location
	s.presentationMu.Unlock()

	log.Printf(ctx, "auth.presentation completed via storage event request_id=%s subject=%s roles=%v", requestID, subject, roles)
	return nil
}

// parseVPClaims accepts either an LDP-VP (JSON-LD object) or a JWT-VP
// (compact serialization) and returns its claims as a generic map.
func parseVPClaims(vp []byte) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(string(vp))
	if trimmed == "" {
		return nil, fmt.Errorf("empty VP")
	}
	if strings.HasPrefix(trimmed, "{") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
			return nil, fmt.Errorf("invalid LDP-VP JSON: %w", err)
		}
		return m, nil
	}
	return parseUnverifiedJWTClaims(trimmed)
}

func extractRequestObjectFromCVResponse(body []byte) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return ""
	}

	var cvObj struct {
		RequestURI string `json:"request_uri"`
	}
	if err := json.Unmarshal(body, &cvObj); err == nil && strings.TrimSpace(cvObj.RequestURI) != "" {
		return strings.TrimSpace(cvObj.RequestURI)
	}

	var cvString string
	if err := json.Unmarshal(body, &cvString); err == nil && strings.TrimSpace(cvString) != "" {
		return strings.TrimSpace(cvString)
	}

	return raw
}

// submitProofToCV submits the VP to the credential-verification service for verification and proof completion.
func (s *authSvc) submitProofToCV(ctx context.Context, requestID string, vpToken string) error {
	if s.cvURL == "" {
		return fmt.Errorf("credential verification URL not configured")
	}

	form := url.Values{}
	form.Set("vp_token", vpToken)
	form.Set("presentation_submission", "{}")

	cvProofURL := fmt.Sprintf("%s/v1/tenants/tenant_space/presentation/proof/%s", s.cvURL, url.PathEscape(requestID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cvProofURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("cv proof submission failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-tenantId", s.presentationNamespace)
	req.Header.Set("x-key", s.presentationKey)
	if s.presentationDid != "" {
		req.Header.Set("x-did", s.presentationDid)
	}
	if s.presentationGroup != "" {
		req.Header.Set("x-groupId", s.presentationGroup)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cv proof submission failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cv returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (s *authSvc) acceptHydraLoginAndConsent(ctx context.Context, challenge, subject string, roles []string) (string, error) {
	loginReq := hydraLoginAcceptReq{
		Subject:     subject,
		Remember:    true,
		RememberFor: 3600,
		Context: map[string]any{
			"roles":  roles,
			"source": "wallet_vp",
		},
	}

	var loginOut hydraAcceptResp
	if err := s.hydraPutJSON(ctx, "/admin/oauth2/auth/requests/login/accept", url.Values{"login_challenge": {challenge}}, loginReq, &loginOut); err != nil {
		return "", err
	}
	if loginOut.RedirectTo == "" {
		return "", fmt.Errorf("hydra login accept returned empty redirect")
	}

	redirectURL, err := url.Parse(loginOut.RedirectTo)
	if err != nil {
		return "", err
	}
	consentChallenge := strings.TrimSpace(redirectURL.Query().Get("consent_challenge"))
	if consentChallenge == "" {
		return loginOut.RedirectTo, nil
	}

	var consentReq hydraConsentRequest
	if err := s.hydraGetJSON(ctx, "/admin/oauth2/auth/requests/consent", url.Values{"consent_challenge": {consentChallenge}}, &consentReq); err != nil {
		return "", err
	}

	consentBody := hydraConsentAcceptReq{
		GrantScope:               consentReq.RequestedScope,
		GrantAccessTokenAudience: consentReq.RequestedAccessTokenAudience,
		Remember:                 true,
		RememberFor:              3600,
	}
	consentBody.Session.AccessToken = map[string]any{"roles": roles}
	consentBody.Session.IDToken = map[string]any{"roles": roles}

	var consentOut hydraAcceptResp
	if err := s.hydraPutJSON(ctx, "/admin/oauth2/auth/requests/consent/accept", url.Values{"consent_challenge": {consentChallenge}}, consentBody, &consentOut); err != nil {
		return "", err
	}
	if consentOut.RedirectTo == "" {
		return "", fmt.Errorf("hydra consent accept returned empty redirect")
	}
	return consentOut.RedirectTo, nil
}

// Consent auto-accepts a Hydra consent challenge for the first-party DCS
// client. The wallet VP already established the user's identity and roles
// during login accept; the consent step exists only because Hydra's flow
// always redirects through consentURL on first login.
func (s *authSvc) Consent(ctx context.Context, p *genauth.ConsentPayload) (*genauth.ConsentResult, error) {
	challenge := strings.TrimSpace(p.ConsentChallenge)
	if challenge == "" {
		return nil, goa.PermanentError("bad_request", "consent_challenge is required")
	}

	var consentReq hydraConsentRequest
	if err := s.hydraGetJSON(ctx, "/admin/oauth2/auth/requests/consent", url.Values{"consent_challenge": {challenge}}, &consentReq); err != nil {
		return nil, goa.PermanentError("unauthorized", "hydra consent fetch: %v", err)
	}

	var roles []string
	if consentReq.Context != nil {
		if raw, ok := consentReq.Context["roles"]; ok {
			if arr, ok := raw.([]any); ok {
				for _, r := range arr {
					if s, ok := r.(string); ok {
						roles = append(roles, s)
					}
				}
			}
		}
	}

	consentBody := hydraConsentAcceptReq{
		GrantScope:               consentReq.RequestedScope,
		GrantAccessTokenAudience: consentReq.RequestedAccessTokenAudience,
		Remember:                 true,
		RememberFor:              3600,
	}
	consentBody.Session.AccessToken = map[string]any{"roles": roles}
	consentBody.Session.IDToken = map[string]any{"roles": roles}

	var consentOut hydraAcceptResp
	if err := s.hydraPutJSON(ctx, "/admin/oauth2/auth/requests/consent/accept", url.Values{"consent_challenge": {challenge}}, consentBody, &consentOut); err != nil {
		return nil, goa.PermanentError("unauthorized", "hydra consent accept: %v", err)
	}
	if consentOut.RedirectTo == "" {
		return nil, goa.PermanentError("unauthorized", "hydra consent accept returned empty redirect")
	}

	return &genauth.ConsentResult{Location: consentOut.RedirectTo}, nil
}

func (s *authSvc) hydraGetJSON(ctx context.Context, path string, q url.Values, out any) error {
	return s.doHydraJSON(ctx, http.MethodGet, path, q, nil, out)
}

func (s *authSvc) hydraPutJSON(ctx context.Context, path string, q url.Values, body any, out any) error {
	return s.doHydraJSON(ctx, http.MethodPut, path, q, body, out)
}

func (s *authSvc) doHydraJSON(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	adminURL := strings.TrimRight(strings.TrimSpace(os.Getenv("HYDRA_ADMIN_URL")), "/")
	if adminURL == "" {
		adminURL = "http://localhost:30085"
	}

	u := adminURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = strings.NewReader(string(payload))
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("hydra %s %s failed: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(errMsg)))
	}

	if out == nil {
		return nil
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("hydra %s %s read body: %w", method, path, err)
	}
	if len(msg) == 0 {
		return nil
	}
	return json.Unmarshal(msg, out)
}

func extractRolesFromClaims(claims map[string]interface{}) []string {
	set := map[string]struct{}{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			set[v] = struct{}{}
		}
	}
	addAny := func(v interface{}) {
		switch t := v.(type) {
		case string:
			add(t)
		case []interface{}:
			for _, e := range t {
				if s, ok := e.(string); ok {
					add(s)
				}
			}
		}
	}

	addAny(getAny(claims, "credentialSubject.role"))
	addAny(getAny(claims, "vc.credentialSubject.role"))

	if roles, ok := claims["roles"].([]interface{}); ok {
		for _, r := range roles {
			if s, ok := r.(string); ok {
				add(s)
			}
		}
	}

	// Walk verifiableCredential at both top-level (LDP-VP JSON) and nested
	// under "vp" (JWT-VP claims). Each entry may be a JWT compact string or
	// a JSON-LD VC object.
	walkVCs := func(vcs []interface{}) {
		for _, item := range vcs {
			switch v := item.(type) {
			case string:
				if vcClaims, err := parseUnverifiedJWTClaims(v); err == nil {
					addAny(getAny(vcClaims, "credentialSubject.role"))
					addAny(getAny(vcClaims, "vc.credentialSubject.role"))
				}
			case map[string]interface{}:
				addAny(getAny(v, "credentialSubject.role"))
			}
		}
	}
	if vcs, ok := claims["verifiableCredential"].([]interface{}); ok {
		walkVCs(vcs)
	}
	if vp, ok := claims["vp"].(map[string]interface{}); ok {
		if vcs, ok := vp["verifiableCredential"].([]interface{}); ok {
			walkVCs(vcs)
		}
	}

	roles := make([]string, 0, len(set))
	for role := range set {
		roles = append(roles, role)
	}
	return roles
}

// extractSubjectFromVP returns the holder/subject DID from a VP claims map,
// covering JWT-VP (iss / sub / vp.holder) and LDP-VP (holder / verifiableCredential[].credentialSubject.id).
//
// Per W3C VC-JWT, a JWT-VP carries the holder DID in the `iss` claim; `sub`
// is typically absent or set to the verifier audience, so `iss` takes
// precedence.
func extractSubjectFromVP(claims map[string]interface{}) string {
	if s := strings.TrimSpace(getString(claims, "iss")); s != "" {
		return s
	}
	if s := strings.TrimSpace(getString(claims, "holder")); s != "" {
		return s
	}
	if s := strings.TrimSpace(getString(claims, "vp.holder")); s != "" {
		return s
	}
	if s := strings.TrimSpace(getString(claims, "sub")); s != "" {
		return s
	}
	if vcs, ok := claims["verifiableCredential"].([]interface{}); ok {
		for _, item := range vcs {
			if m, ok := item.(map[string]interface{}); ok {
				if s := strings.TrimSpace(getString(m, "credentialSubject.id")); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func truncateForLog(s string) string {
	if len(s) > 50 {
		return s[:50] + "..."
	}
	return s
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

// parseUnverifiedJWTClaims extracts claims from a JWT without verifying the signature
func parseUnverifiedJWTClaims(jwtStr string) (map[string]interface{}, error) {
	parts := strings.Split(jwtStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	claimsB64 := parts[1]
	// Add padding if needed
	switch len(claimsB64) % 4 {
	case 2:
		claimsB64 += "=="
	case 3:
		claimsB64 += "="
	}

	decoded, err := base64.StdEncoding.DecodeString(claimsB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}
	return claims, nil
}

// getString retrieves a string value from nested map using dot notation
// e.g., "credentialSubject.role" traverses claims["credentialSubject"]["role"]
func getString(data map[string]interface{}, path string) string {
	if s, ok := getAny(data, path).(string); ok {
		return s
	}
	return ""
}

// getAny traverses a nested map using dot notation and returns the raw value.
func getAny(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(data)
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

// firstNonEmpty returns the first non-empty string from the given arguments
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}
