package service

import (
	"context"
	"fmt"

	credentialissuance "digital-contracting-service/gen/credential_issuance"
	"digital-contracting-service/internal/auth"
	"digital-contracting-service/internal/ocmw"

	"goa.design/clue/log"
)

// credentialIssuanceSvc implements the generated credential_issuance.Service interface.
type credentialIssuanceSvc struct {
	auth.JWTAuthenticator
	issuance *ocmw.IssuanceClient
}

// NewCredentialIssuance returns the CredentialIssuance service backed by the
// given OCM-W issuance client.
func NewCredentialIssuance(jwtAuth auth.JWTAuthenticator, issuance *ocmw.IssuanceClient) credentialissuance.Service {
	return &credentialIssuanceSvc{
		JWTAuthenticator: jwtAuth,
		issuance:         issuance,
	}
}

// IssueRoleCredential creates an OID4VCI credential offer for the given holder
// DID and DCS role. The returned offer_uri should be handed to the wallet
// holder out of band.
func (s *credentialIssuanceSvc) IssueRoleCredential(ctx context.Context, p *credentialissuance.IssueRoleCredentialRequest) (*credentialissuance.IssueRoleCredentialResponse, error) {
	log.Printf(ctx, "credential_issuance.issue_role_credential holder_did=%s role=%s", p.HolderDid, p.Role)

	offerResp, err := s.issuance.CreateOffer(ctx, ocmw.CredentialOfferRequest{
		HolderDID:      p.HolderDid,
		CredentialType: "DCSRoleCredential",
		Claims: map[string]interface{}{
			"id":   p.HolderDid,
			"role": p.Role,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("issuance service error: %w", err)
	}

	return &credentialissuance.IssueRoleCredentialResponse{
		OfferURI: offerResp.OfferURI,
		Offer:    offerResp.Raw,
	}, nil
}
