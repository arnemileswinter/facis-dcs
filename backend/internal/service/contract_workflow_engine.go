package service

import (
	"context"
	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	templaterepository "digital-contracting-service/gen/template_repository"
	"digital-contracting-service/internal/auth"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/eventbus"
	"digital-contracting-service/internal/base/eventbus/eventbuschannel"
	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/contractworkflowengine/datatype/actionflag"
	"digital-contracting-service/internal/contractworkflowengine/db"
	"digital-contracting-service/internal/middleware"
	"time"

	"github.com/jmoiron/sqlx"
	"goa.design/clue/log"
)

type contractWorkflowEnginesrvc struct {
	DB       *sqlx.DB
	EventBus eventbus.EventBus
	CRepo    db.ContractRepo
	RTRepo   db.ReviewTaskRepo
	ATRepo   db.ApprovalTaskRepo
	NRepo    db.NegotiationRepo
	auth.JWTAuthenticator
}

func messageHandler(data []byte) {

}

func NewContractWorkflowEngine(db *sqlx.DB, jwtAuth auth.JWTAuthenticator, eb eventbus.EventBus,
	cRepo db.ContractRepo, rtRepo db.ReviewTaskRepo, atRepo db.ApprovalTaskRepo,
	nRepo db.NegotiationRepo) (contractworkflowengine.Service, error) {

	err := eb.SubscribeAsync(eventbuschannel.ContractWorkflowEngine.String(), messageHandler)
	if err != nil {
		return nil, err
	}

	return &contractWorkflowEnginesrvc{
		JWTAuthenticator: jwtAuth,
		DB:               db,
		EventBus:         eb,
		CRepo:            cRepo,
		RTRepo:           rtRepo,
		ATRepo:           atRepo,
		NRepo:            nRepo,
	}, nil
}

func (s *contractWorkflowEnginesrvc) Create(ctx context.Context, req *contractworkflowengine.ContractCreateRequest) (res *contractworkflowengine.ContractCreateResponse, err error) {

	did, err := base.GetDID()
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	cmd := command.CreateCmd{
		DID:         *did,
		TemplateDID: req.Did,
		CreatedBy:   middleware.GetUsername(ctx),
	}
	createHandler := command.Creator{
		Ctx:   ctx,
		DB:    s.DB,
		CRepo: s.CRepo,
	}
	err = createHandler.Handle(cmd)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	return &contractworkflowengine.ContractCreateResponse{
		Did: *did,
	}, nil
}

func (s *contractWorkflowEnginesrvc) Update(ctx context.Context, req *contractworkflowengine.ContractUpdateRequest) (res *contractworkflowengine.ContractUpdateResponse, err error) {

	updatedAt, err := time.Parse(time.RFC3339, req.UpdatedAt)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	metaData, err := datatype.NewJSON(req.ContractData)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	cmd := command.UpdateCmd{
		DID:             req.Did,
		ContractVersion: req.ContractVersion,
		UpdatedAt:       updatedAt,
		Name:            req.Name,
		Description:     req.Description,
		ContractData:    &metaData,
	}
	handler := command.Updater{
		Ctx:   ctx,
		DB:    s.DB,
		CRepo: s.CRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	return &contractworkflowengine.ContractUpdateResponse{
		Did: req.Did,
	}, nil
}

func (s *contractWorkflowEnginesrvc) Submit(ctx context.Context, req *contractworkflowengine.ContractSubmitRequest) (res *contractworkflowengine.ContractSubmitResponse, err error) {

	updatedAt, err := time.Parse(time.RFC3339, req.UpdatedAt)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	var actionFlag *actionflag.ActionFlag
	if req.ForwardTo != nil {
		flag, err := actionflag.NewActionFlag(*req.ForwardTo)
		if err != nil {
			return nil, templaterepository.MakeInternalError(err)
		}
		actionFlag = &flag
	}

	cmd := command.SubmitCmd{
		DID:         req.Did,
		UpdatedAt:   updatedAt,
		SubmittedBy: middleware.GetUsername(ctx),
		ActionFlag:  actionFlag,
		Reviewer:    req.Reviewers,
		Approver:    req.Approver,
	}
	handler := command.Submitter{
		Ctx:    ctx,
		DB:     s.DB,
		CRepo:  s.CRepo,
		RTRepo: s.RTRepo,
		ATRepo: s.ATRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		return nil, templaterepository.MakeInternalError(err)
	}

	return &contractworkflowengine.ContractSubmitResponse{
		Did: req.Did,
	}, nil
}

func (s *contractWorkflowEnginesrvc) Retrieve(ctx context.Context, request *contractworkflowengine.ContractRetrieveRequest) (res *contractworkflowengine.ContractRetrieveResponse, err error) {
	panic("implement me")
}

func (s *contractWorkflowEnginesrvc) RetrieveByID(ctx context.Context, request *contractworkflowengine.ContractRetrieveByIDRequest) (res *contractworkflowengine.ContractRetrieveByIDResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (s *contractWorkflowEnginesrvc) Verify(ctx context.Context, req *contractworkflowengine.ContractVerifyRequest) (res *contractworkflowengine.ContractVerifyResponse, err error) {
	updatedAt, err := time.Parse(time.RFC3339, req.UpdatedAt)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	cmd := command.VerifyCmd{
		DID:        req.Did,
		UpdatedAt:  updatedAt,
		VerifiedBy: middleware.GetUsername(ctx),
	}
	handler := command.Verifier{
		Ctx:    ctx,
		DB:     s.DB,
		CRepo:  s.CRepo,
		RTRepo: s.RTRepo,
	}
	err = handler.Handle(cmd)
	if err != nil {
		return nil, contractworkflowengine.MakeInternalError(err)
	}

	return &contractworkflowengine.ContractVerifyResponse{
		Did: req.Did,
	}, nil
}

func (s *contractWorkflowEnginesrvc) Negotiate(ctx context.Context, req *contractworkflowengine.NegotiatePayload) (res string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.negotiate")
	return
}

func (s *contractWorkflowEnginesrvc) Respond(ctx context.Context, req *contractworkflowengine.RespondPayload) (res string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.respond")
	return
}

func (s *contractWorkflowEnginesrvc) Review(ctx context.Context, req *contractworkflowengine.ReviewPayload) (res any, err error) {
	log.Printf(ctx, "contractWorkflowEngine.review")
	return
}

func (s *contractWorkflowEnginesrvc) Search(ctx context.Context, req *contractworkflowengine.SearchPayload) (res []any, err error) {
	log.Printf(ctx, "contractWorkflowEngine.search")
	return
}

func (s *contractWorkflowEnginesrvc) Approve(ctx context.Context, req *contractworkflowengine.ApprovePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.approve")
	return
}

func (s *contractWorkflowEnginesrvc) Reject(ctx context.Context, req *contractworkflowengine.RejectPayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.reject")
	return
}

func (s *contractWorkflowEnginesrvc) Store(ctx context.Context, req *contractworkflowengine.StorePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.store")
	return
}

func (s *contractWorkflowEnginesrvc) Terminate(ctx context.Context, req *contractworkflowengine.TerminatePayload) (res int, err error) {
	log.Printf(ctx, "contractWorkflowEngine.terminate")
	return
}

func (s *contractWorkflowEnginesrvc) Audit(ctx context.Context, req *contractworkflowengine.AuditPayload) (res []string, err error) {
	log.Printf(ctx, "contractWorkflowEngine.audit")
	return
}
