// Package machineidentity is the registry of non-human callers: the SRS Table 5
// System Users and the Contract Target Systems. Each authenticates with its own
// OAuth2 client provisioned in Hydra, so a credential can be issued once, shown
// once and rotated without a redeploy (ADR-27).
package machineidentity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Identity is one registered machine caller.
//
// The secret is deliberately absent: Hydra holds it hashed and cannot return
// it, which is what makes "shown once" a property of the system rather than a
// promise. SecretIssuedAt only lets an operator see how old a credential is.
type Identity struct {
	ID             string     `db:"id"`
	Name           string     `db:"name"`
	OAuthClientID  string     `db:"oauth_client_id"`
	ParticipantDID string     `db:"participant_did"`
	RolesJSON      string     `db:"roles"`
	Description    *string    `db:"description"`
	Enabled        bool       `db:"enabled"`
	SecretIssuedAt *time.Time `db:"secret_issued_at"`
	CreatedBy      string     `db:"created_by"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// Roles decodes the stored role list. Authority is read from here by client_id
// and never from the token, so a caller cannot widen its own reach.
func (i Identity) Roles() ([]string, error) {
	trimmed := strings.TrimSpace(i.RolesJSON)
	if trimmed == "" {
		return nil, nil
	}
	var roles []string
	if err := json.Unmarshal([]byte(trimmed), &roles); err != nil {
		return nil, fmt.Errorf("machine identity %q has an unreadable role list: %w", i.Name, err)
	}
	return roles, nil
}

// EncodeRoles renders a role list for storage.
func EncodeRoles(roles []string) (string, error) {
	if len(roles) == 0 {
		return "", fmt.Errorf("a machine identity needs at least one role")
	}
	encoded, err := json.Marshal(roles)
	if err != nil {
		return "", fmt.Errorf("could not encode the role list: %w", err)
	}
	return string(encoded), nil
}

// Repo stores registered machine identities.
type Repo interface {
	List(ctx context.Context) ([]Identity, error)
	Read(ctx context.Context, id string) (*Identity, error)
	// FindByClientID resolves the caller behind an access token's client_id
	// claim. It is on the hot path of every machine-authenticated request.
	FindByClientID(ctx context.Context, clientID string) (*Identity, error)
	Create(ctx context.Context, data Identity) (*Identity, error)
	Update(ctx context.Context, data Identity) (*Identity, error)
	Delete(ctx context.Context, id string) error
	// TouchSecretIssuedAt records that a new secret was handed out.
	TouchSecretIssuedAt(ctx context.Context, id string, at time.Time) error
	// Upsert seeds a declaratively configured identity, so a deployment can
	// bootstrap the callers it needs before anyone logs in to create them.
	Upsert(ctx context.Context, data Identity) error
}
