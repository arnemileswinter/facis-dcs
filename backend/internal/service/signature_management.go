package service

import (
	"context"
	signaturemanagement "digital-contracting-service/gen/signature_management"
	"digital-contracting-service/internal/auth"

	"goa.design/clue/log"
)

type signatureManagementsrvc struct {
	auth.JWTAuthenticator
}

func NewSignatureManagement(jwtAuth auth.JWTAuthenticator) signaturemanagement.Service {
	return &signatureManagementsrvc{JWTAuthenticator: jwtAuth}
}

func (s *signatureManagementsrvc) Retrieve(ctx context.Context, p *signaturemanagement.RetrievePayload) (res any, err error) {
	log.Printf(ctx, "signatureManagement.retrieve")
	return
}

func (s *signatureManagementsrvc) Verify(ctx context.Context, p *signaturemanagement.VerifyPayload) (res any, err error) {
	log.Printf(ctx, "signatureManagement.verify")
	return
}

func (s *signatureManagementsrvc) Apply(ctx context.Context, p *signaturemanagement.ApplyPayload) (res int, err error) {
	log.Printf(ctx, "signatureManagement.apply")
	return
}

func (s *signatureManagementsrvc) Validate(ctx context.Context, p *signaturemanagement.ValidatePayload) (res any, err error) {
	log.Printf(ctx, "signatureManagement.validate")
	return
}

func (s *signatureManagementsrvc) Revoke(ctx context.Context, p *signaturemanagement.RevokePayload) (res int, err error) {
	log.Printf(ctx, "signatureManagement.revoke")
	return
}

func (s *signatureManagementsrvc) Audit(ctx context.Context, p *signaturemanagement.AuditPayload) (res []string, err error) {
	log.Printf(ctx, "signatureManagement.audit")
	return
}

func (s *signatureManagementsrvc) Compliance(ctx context.Context, p *signaturemanagement.CompliancePayload) (res any, err error) {
	log.Printf(ctx, "signatureManagement.compliance")
	return
}
