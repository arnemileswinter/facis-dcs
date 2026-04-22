package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// holds OIDC provider configuration
type OIDCConfig struct {
	// Example: https://hydra.example.com/ or https://issuer.example.com/realms/dcs
	IssuerURL string
	// Example: "dcs-service". The token must identify this client via azp or aud.
	ClientID string
}

// validate JWT tokens from OIDC providers
type OIDCValidator struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   OIDCConfig
}

// connects to the OIDC provider to get public keys
func NewOIDCValidator(ctx context.Context, config OIDCConfig) (*OIDCValidator, error) {
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	// Defer client binding checks to ValidateToken so both Keycloak-style azp
	// claims and standard aud claims work.
	verifier := provider.Verifier(&oidc.Config{
		ClientID:          config.ClientID,
		SkipClientIDCheck: true,
	})

	return &OIDCValidator{
		provider: provider,
		verifier: verifier,
		config:   config,
	}, nil
}

// TokenInfo holds the validated identity extracted from a JWT.
type TokenInfo struct {
	Roles         []string
	Username      string
	ParticipantID string
}

// ValidateToken verifies the token signature, issuer, and azp claim, then
// returns the caller's roles and username.
func (v *OIDCValidator) ValidateToken(ctx context.Context, token string) (*TokenInfo, error) {
	idToken, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse token claims: %w", err)
	}

	if !matchesClientID(claims, v.config.ClientID) {
		return nil, fmt.Errorf("token is not bound to client ID %q", v.config.ClientID)
	}

	username, _ := claims["preferred_username"].(string)
	if username == "" {
		username, _ = claims["sub"].(string)
	}
	// This claim is optional and provider-specific.
	participantID, _ := claims["participant-id"].(string)

	return &TokenInfo{
		Roles:         extractRoles(claims),
		Username:      username,
		ParticipantID: participantID,
	}, nil
}

// extractRoles extracts DCS roles from OIDC token claims.
func extractRoles(claims map[string]interface{}) []string {
	if roles := toStringSlice(claims["roles"]); len(roles) > 0 {
		return roles
	}
	// Hydra session.access_token claims land under `ext`.
	if ext, ok := claims["ext"].(map[string]interface{}); ok {
		if roles := toStringSlice(ext["roles"]); len(roles) > 0 {
			return roles
		}
	}
	if roles := extractResourceAccessRoles(claims); len(roles) > 0 {
		return roles
	}
	if roles := extractRealmRoles(claims); len(roles) > 0 {
		return roles
	}
	if scope, ok := claims["scope"].(string); ok && scope != "" {
		return strings.Fields(scope)
	}
	return []string{}
}

func extractResourceAccessRoles(claims map[string]interface{}) []string {
	ra, ok := claims["resource_access"].(map[string]interface{})
	if !ok {
		return []string{}
	}
	azp, ok := claims["azp"].(string)
	if !ok {
		return []string{}
	}
	client, ok := ra[azp].(map[string]interface{})
	if !ok {
		return []string{}
	}
	if roles := toStringSlice(client["roles"]); len(roles) > 0 {
		return roles
	}
	return []string{}
}

func extractRealmRoles(claims map[string]interface{}) []string {
	realmAccess, ok := claims["realm_access"].(map[string]interface{})
	if !ok {
		return []string{}
	}
	return toStringSlice(realmAccess["roles"])
}

func matchesClientID(claims map[string]interface{}, clientID string) bool {
	if azp, _ := claims["azp"].(string); azp != "" {
		return azp == clientID
	}
	// Ory Hydra access tokens carry the OAuth2 client in `client_id`
	// rather than the OIDC `azp` claim.
	if cid, _ := claims["client_id"].(string); cid != "" {
		return cid == clientID
	}
	switch aud := claims["aud"].(type) {
	case string:
		return aud == clientID
	case []interface{}:
		for _, item := range aud {
			if audience, ok := item.(string); ok && audience == clientID {
				return true
			}
		}
	}
	return false
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Expected format: "Authorization: Bearer <token>"
func ExtractBearerToken(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", fmt.Errorf("invalid authorization header format")
	}
	return strings.TrimPrefix(authHeader, bearerPrefix), nil
}

// unexported key type to avoid context key collisions.
type authCtxKey struct{}

// AuthContext carries the validated caller identity through the request context.
type AuthContext struct {
	Roles         []string
	Username      string
	ParticipantID string
}

// GetRoles extracts roles from the request context.
func GetRoles(ctx context.Context) []string {
	if ac, ok := ctx.Value(authCtxKey{}).(AuthContext); ok {
		return ac.Roles
	}
	return []string{}
}

// GetUsername extracts the authenticated username from the request context.
func GetUsername(ctx context.Context) string {
	if ac, ok := ctx.Value(authCtxKey{}).(AuthContext); ok {
		return ac.Username
	}
	return ""
}

// GetParticipantID extracts the authenticated participant ID from the request context.
func GetParticipantID(ctx context.Context) string {
	if ac, ok := ctx.Value(authCtxKey{}).(AuthContext); ok {
		return ac.ParticipantID
	}
	return ""
}

// HasRole checks if the context contains a specific role.
func HasRole(ctx context.Context, requiredRole string) bool {
	for _, role := range GetRoles(ctx) {
		if role == requiredRole {
			return true
		}
	}
	return false
}

// InjectAuthContext injects the validated identity into the request context.
func InjectAuthContext(ctx context.Context, roles []string, username string, participantID string) context.Context {
	return context.WithValue(ctx, authCtxKey{}, AuthContext{Roles: roles, Username: username, ParticipantID: participantID})
}
