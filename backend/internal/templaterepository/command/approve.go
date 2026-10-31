package command

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/datatype/userrole"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/base/templatestructure"
	"digital-contracting-service/internal/base/validation"
	"digital-contracting-service/internal/templaterepository/datatype/approvaltaskstate"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/db"
	templateevents "digital-contracting-service/internal/templaterepository/event"
)

type ApproveCmd struct {
	DID           string
	UpdatedAt     time.Time
	ApprovedBy    string
	DecisionNotes []string
	HolderDID     string
	UserRoles     userrole.UserRoles
}

type Approver struct {
	DB     *sqlx.DB
	CTRepo db.ContractTemplateRepo
	ATRepo db.ApprovalTaskRepo
}

func (h *Approver) Handle(ctx context.Context, cmd ApproveCmd) error {

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {
			log.Printf("could not rollback transaction: %v", err)
		}
	}(tx)

	processData, err := h.CTRepo.ReadProcessDataByDID(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	// Optimistic concurrency: reject if the caller's view of the template is
	// older than what's stored (see command package doc / ADR-0007). Templates
	// aren't peer-synced, so unlike contractworkflowengine there's no
	// local-vs-remote distinction in the error message here.
	if cmd.UpdatedAt.Unix() < processData.ContentUpdatedAt.Unix() {
		return fmt.Errorf("contract template %w, please reload", base.ErrUpdatedElsewhere)
	}

	if processData.State != contracttemplatestate.Reviewed.String() {
		return errors.New("invalid contract template state")
	}

	valid, err := h.ATRepo.IsValidApprover(ctx, tx, cmd.DID, cmd.ApprovedBy)
	if err != nil {
		return err
	}

	if !valid {
		return errors.New("invalid user")
	}

	// DCS-FR-TR-26: the required templates can have changed since submission.
	if err := templatestructure.CheckTemplate(ctx, tx, cmd.DID); err != nil {
		return err
	}

	err = h.ATRepo.UpdateState(ctx, tx, cmd.DID, cmd.ApprovedBy, approvaltaskstate.Approved.String())
	if err != nil {
		return fmt.Errorf("could not update approval task state: %w", err)
	}

	err = h.CTRepo.UpdateState(ctx, tx, cmd.DID, contracttemplatestate.Approved.String())
	if err != nil {
		return fmt.Errorf("could not update current template state: %w", err)
	}

	// DCS-FR-TR-10: the template policy audit at approval sets the regulatory compliance status.
	template, err := h.CTRepo.ReadDataByID(ctx, tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read template data: %w", err)
	}
	findings, err := validation.AuditTemplatePolicies(template.TemplateData, validation.TemplatePolicyAuditMetadata{
		DID:          template.DID,
		TemplateType: template.TemplateType,
		State:        template.State,
	})
	if err != nil {
		return fmt.Errorf("could not audit template policies: %w", err)
	}
	if err := h.CTRepo.UpdateComplianceStatus(ctx, tx, cmd.DID, ComplianceStatus(findings)); err != nil {
		return fmt.Errorf("could not update compliance status: %w", err)
	}

	evt := templateevents.ApproveEvent{
		DID:           cmd.DID,
		Version:       processData.Version,
		ApprovedBy:    cmd.ApprovedBy,
		DecisionNotes: cmd.DecisionNotes,
		OccurredAt:    time.Now().UTC(),
		HolderDID:     cmd.HolderDID,
		UserRoles:     cmd.UserRoles,
	}
	err = event.Create(ctx, tx, evt, componenttype.ContractTemplateRepo)
	if err != nil {
		return fmt.Errorf("could not create event: %w", err)
	}

	return tx.Commit()
}

// ComplianceStatus is NON_COMPLIANT when the template policy audit reports an
// error, else COMPLIANT.
func ComplianceStatus(findings []validation.PolicyFinding) string {
	for _, finding := range findings {
		if finding.Severity == validation.SeverityError {
			return "NON_COMPLIANT"
		}
	}
	return "COMPLIANT"
}
