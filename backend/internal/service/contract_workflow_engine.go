package service

import (
	"context"
	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	"digital-contracting-service/internal/auth"

	"goa.design/clue/log"
)

type contractWorkflowEnginesrvc struct {
	auth.JWTAuthenticator
}

func NewContractWorkflowEngine(jwtAuth auth.JWTAuthenticator) contractworkflowengine.Service {
	return &contractWorkflowEnginesrvc{JWTAuthenticator: jwtAuth}
}

func (s *contractWorkflowEnginesrvc) Create(ctx context.Context, p *contractworkflowengine.ContractCreateRequest) (res *contractworkflowengine.ContractCreateResponse, err error) {
	log.Printf(ctx, "contractWorkflowEngine.create")
	return
}

func (s *contractWorkflowEnginesrvc) Submit(ctx context.Context, p *contractworkflowengine.SubmitPayload) (res string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.submit")
	return
}

func (s *contractWorkflowEnginesrvc) Negotiate(ctx context.Context, p *contractworkflowengine.NegotiatePayload) (res string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.negotiate")
	return
}

func (s *contractWorkflowEnginesrvc) Respond(ctx context.Context, p *contractworkflowengine.RespondPayload) (res string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.respond")
	return
}

func (s *contractWorkflowEnginesrvc) Review(ctx context.Context, p *contractworkflowengine.ReviewPayload) (res any, err error) {
	log.Printf(ctx, "contractWorkflowEngine.review")
	return
}

func (s *contractWorkflowEnginesrvc) Retrieve(ctx context.Context, p *contractworkflowengine.RetrievePayload) (res any, err error) {
	log.Printf(ctx, "contractWorkflowEngine.retrieve")
	return
}

func (s *contractWorkflowEnginesrvc) Search(ctx context.Context, p *contractworkflowengine.SearchPayload) (res []any, err error) {
	log.Printf(ctx, "contractWorkflowEngine.search")
	return
}

func (s *contractWorkflowEnginesrvc) Approve(ctx context.Context, p *contractworkflowengine.ApprovePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.approve")
	return
}

func (s *contractWorkflowEnginesrvc) Reject(ctx context.Context, p *contractworkflowengine.RejectPayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.reject")
	return
}

func (s *contractWorkflowEnginesrvc) Store(ctx context.Context, p *contractworkflowengine.StorePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.store")
	return
}

func (s *contractWorkflowEnginesrvc) Terminate(ctx context.Context, p *contractworkflowengine.TerminatePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.terminate")
	return
}

func (s *contractWorkflowEnginesrvc) Audit(ctx context.Context, p *contractworkflowengine.AuditPayload) (res []string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.audit")
	return
}
