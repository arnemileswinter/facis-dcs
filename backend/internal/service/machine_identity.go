package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	"digital-contracting-service/internal/auth/hydra"
	"digital-contracting-service/internal/auth/machineidentity"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/middleware"

	"github.com/google/uuid"
)

// MachineCredentialIssuer provisions the OAuth2 clients machine callers
// authenticate with. Hydra keeps the secret hashed, so an issued secret is
// readable exactly once — at the moment it is returned here (ADR-27).
type MachineCredentialIssuer interface {
	CreateMachineClient(ctx context.Context, clientID, name string) (*hydra.OAuth2Client, error)
	RotateMachineClientSecret(ctx context.Context, clientID string) (string, error)
	DeleteMachineClient(ctx context.Context, clientID string) error
}

func machineIdentityView(identity *machineidentity.Identity) (*contractworkflowengine.MachineIdentity, error) {
	roles, err := identity.Roles()
	if err != nil {
		return nil, err
	}
	view := &contractworkflowengine.MachineIdentity{
		ID:             identity.ID,
		Name:           identity.Name,
		OauthClientID:  identity.OAuthClientID,
		ParticipantDid: identity.ParticipantDID,
		Roles:          roles,
		Enabled:        identity.Enabled,
		Description:    identity.Description,
		CreatedAt:      ptr(identity.CreatedAt.UTC().Format(time.RFC3339)),
		UpdatedAt:      ptr(identity.UpdatedAt.UTC().Format(time.RFC3339)),
	}
	if identity.SecretIssuedAt != nil {
		view.SecretIssuedAt = ptr(identity.SecretIssuedAt.UTC().Format(time.RFC3339))
	}
	return view, nil
}

// validateRoles refuses an unknown role at configuration time. A caller whose
// role does not exist would otherwise authenticate and then be refused
// everywhere, with nothing to say why.
func validateRoles(roles []string) error {
	if len(roles) == 0 {
		return errors.New("a machine identity needs at least one role")
	}
	for _, role := range roles {
		if !userrole.UserRole(strings.TrimSpace(role)).IsValid() {
			return fmt.Errorf("unknown role %q", role)
		}
	}
	return nil
}

func (s *contractWorkflowEnginesrvc) ListMachineIdentities(ctx context.Context, req *contractworkflowengine.MachineIdentityListRequest) (res *contractworkflowengine.MachineIdentityListResponse, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	identities, err := s.MachineIdentities.List(ctx)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	views := make([]*contractworkflowengine.MachineIdentity, 0, len(identities))
	for i := range identities {
		view, err := machineIdentityView(&identities[i])
		if err != nil {
			return nil, contractworkflowengine.MakeInternalError(err)
		}
		views = append(views, view)
	}
	return &contractworkflowengine.MachineIdentityListResponse{Identities: views}, nil
}

func (s *contractWorkflowEnginesrvc) CreateMachineIdentity(ctx context.Context, req *contractworkflowengine.MachineIdentityCreateRequest) (res *contractworkflowengine.MachineIdentityCreateResponse, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	name := strings.TrimSpace(req.Name)
	if name == "" || strings.TrimSpace(req.ParticipantDid) == "" {
		return nil, contractworkflowengine.MakeBadRequest(errors.New("a machine identity needs a name and a participant DID to attribute its actions to"))
	}
	if err := validateRoles(req.Roles); err != nil {
		return nil, contractworkflowengine.MakeBadRequest(err)
	}

	rolesJSON, err := machineidentity.EncodeRoles(req.Roles)
	if err != nil {
		return nil, contractworkflowengine.MakeBadRequest(err)
	}

	// The client id is generated rather than taken from the operator: it is a
	// credential identifier, and letting it be chosen invites collisions with a
	// seeded client and makes an existing identity impersonatable by name.
	clientID := "dcs-machine-" + uuid.NewString()

	issued, err := s.HydraAdmin.CreateMachineClient(ctx, clientID, name)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(fmt.Errorf("could not provision the OAuth2 client: %w", err))
	}

	issuedAt := time.Now().UTC()
	created, err := s.MachineIdentities.Create(ctx, machineidentity.Identity{
		Name:           name,
		OAuthClientID:  clientID,
		ParticipantDID: strings.TrimSpace(req.ParticipantDid),
		RolesJSON:      rolesJSON,
		Description:    req.Description,
		Enabled:        true,
		SecretIssuedAt: &issuedAt,
		CreatedBy:      middleware.GetParticipantID(ctx),
	})
	if err != nil {
		// Leaving the client behind would let a credential exist that no
		// registry entry accounts for, which is exactly what this replaces.
		if cleanup := s.HydraAdmin.DeleteMachineClient(ctx, clientID); cleanup != nil {
			return nil, contractworkflowengine.MakeInternalError(fmt.Errorf("could not register the identity (%w) and could not remove its OAuth2 client: %v", err, cleanup))
		}
		return nil, contractworkflowengine.MakeBadRequest(err)
	}

	view, err := machineIdentityView(created)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	return &contractworkflowengine.MachineIdentityCreateResponse{
		Identity: view,
		Credential: &contractworkflowengine.MachineCredential{
			ClientID:     clientID,
			ClientSecret: issued.Secret,
			TokenURL:     ptr(s.machineTokenURL()),
			IssuedAt:     ptr(issuedAt.Format(time.RFC3339)),
		},
	}, nil
}

func (s *contractWorkflowEnginesrvc) UpdateMachineIdentity(ctx context.Context, req *contractworkflowengine.MachineIdentityUpdateRequest) (res *contractworkflowengine.MachineIdentity, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ParticipantDid) == "" {
		return nil, contractworkflowengine.MakeBadRequest(errors.New("a machine identity needs a name and a participant DID"))
	}
	if err := validateRoles(req.Roles); err != nil {
		return nil, contractworkflowengine.MakeBadRequest(err)
	}

	rolesJSON, err := machineidentity.EncodeRoles(req.Roles)
	if err != nil {
		return nil, contractworkflowengine.MakeBadRequest(err)
	}

	updated, err := s.MachineIdentities.Update(ctx, machineidentity.Identity{
		ID:             req.ID,
		Name:           strings.TrimSpace(req.Name),
		ParticipantDID: strings.TrimSpace(req.ParticipantDid),
		RolesJSON:      rolesJSON,
		Description:    req.Description,
		Enabled:        req.Enabled,
	})
	if err != nil {
		return nil, contractworkflowengine.MakeBadRequest(err)
	}
	if updated == nil {
		return nil, contractworkflowengine.MakeBadRequest(fmt.Errorf("no machine identity %s", req.ID))
	}

	view, err := machineIdentityView(updated)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	return view, nil
}

func (s *contractWorkflowEnginesrvc) DeleteMachineIdentity(ctx context.Context, req *contractworkflowengine.MachineIdentityDeleteRequest) error {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	identity, err := s.MachineIdentities.Read(ctx, req.ID)
	if err != nil {
		return contractworkflowengine.MakeInternalError(err)
	}
	if identity == nil {
		return contractworkflowengine.MakeBadRequest(fmt.Errorf("no machine identity %s", req.ID))
	}

	// The client goes first: a registry row without a client is inert, but a
	// client without a row is a credential nothing accounts for.
	if err := s.HydraAdmin.DeleteMachineClient(ctx, identity.OAuthClientID); err != nil {
		return contractworkflowengine.MakeInternalError(fmt.Errorf("could not remove the OAuth2 client: %w", err))
	}
	if err := s.MachineIdentities.Delete(ctx, req.ID); err != nil {
		return contractworkflowengine.MakeInternalError(err)
	}
	return nil
}

func (s *contractWorkflowEnginesrvc) RotateMachineIdentitySecret(ctx context.Context, req *contractworkflowengine.MachineIdentityRotateRequest) (res *contractworkflowengine.MachineCredential, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	identity, err := s.MachineIdentities.Read(ctx, req.ID)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if identity == nil {
		return nil, contractworkflowengine.MakeBadRequest(fmt.Errorf("no machine identity %s", req.ID))
	}

	secret, err := s.HydraAdmin.RotateMachineClientSecret(ctx, identity.OAuthClientID)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(fmt.Errorf("could not rotate the secret: %w", err))
	}

	issuedAt := time.Now().UTC()
	if err := s.MachineIdentities.TouchSecretIssuedAt(ctx, identity.ID, issuedAt); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	return &contractworkflowengine.MachineCredential{
		ClientID:     identity.OAuthClientID,
		ClientSecret: secret,
		TokenURL:     ptr(s.machineTokenURL()),
		IssuedAt:     ptr(issuedAt.Format(time.RFC3339)),
	}, nil
}

// RotateContractTargetSecret issues the credential a target authenticates its
// deployment callbacks with, creating the OAuth2 client on first use.
func (s *contractWorkflowEnginesrvc) RotateContractTargetSecret(ctx context.Context, req *contractworkflowengine.ContractTargetRotateRequest) (res *contractworkflowengine.MachineCredential, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	defer func() { _ = tx.Rollback() }()

	target, err := s.TargetRepo.ReadTarget(ctx, tx, req.ID)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if target == nil {
		return nil, contractworkflowengine.MakeBadRequest(fmt.Errorf("no target system %s", req.ID))
	}

	var (
		clientID string
		secret   string
	)
	if target.OAuthClientID == nil || strings.TrimSpace(*target.OAuthClientID) == "" {
		clientID = "dcs-target-" + target.ID
		issued, err := s.HydraAdmin.CreateMachineClient(ctx, clientID, target.Name)
		if err != nil {
			return nil, contractworkflowengine.MakeInternalError(fmt.Errorf("could not provision the OAuth2 client: %w", err))
		}
		secret = issued.Secret
	} else {
		clientID = strings.TrimSpace(*target.OAuthClientID)
		secret, err = s.HydraAdmin.RotateMachineClientSecret(ctx, clientID)
		if err != nil {
			return nil, contractworkflowengine.MakeInternalError(fmt.Errorf("could not rotate the secret: %w", err))
		}
	}

	issuedAt := time.Now().UTC()
	if err := s.TargetRepo.SetCredential(ctx, tx, target.ID, clientID, issuedAt); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	return &contractworkflowengine.MachineCredential{
		ClientID:     clientID,
		ClientSecret: secret,
		TokenURL:     ptr(s.machineTokenURL()),
		IssuedAt:     ptr(issuedAt.Format(time.RFC3339)),
	}, nil
}

// machineTokenURL is where an integrator presents the credential. It is handed
// back with the secret so the one-time display carries everything needed to
// configure the caller.
func (s *contractWorkflowEnginesrvc) machineTokenURL() string {
	return strings.TrimRight(s.HydraPublicIssuerURL, "/") + "/oauth2/token"
}
