package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/templaterepository/datatype/actionflag"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/datatype/reviewtaskstate"
	"digital-contracting-service/internal/templaterepository/db"
	templateevents "digital-contracting-service/internal/templaterepository/event"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type SubmitCmd struct {
	DID         string
	UpdatedAt   time.Time
	SubmittedBy string
	ActionFlag  *actionflag.ActionFlag
	Comments    []string
	Reviewer    []string
	Approver    *string
}

type Submitter struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CTRepo db.ContractTemplateRepo
	RTRepo db.ReviewTaskRepo
	ATRepo db.ApprovalTaskRepo
}

func createTasks(tx *sqlx.Tx, rtRepo db.ReviewTaskRepo, atRepo db.ApprovalTaskRepo, cmd SubmitCmd) error {
	for _, reviewer := range cmd.Reviewer {
		reviewTask := db.ReviewTaskData{
			DID:       cmd.DID,
			Reviewer:  reviewer,
			State:     reviewtaskstate.Open.String(),
			CreatedBy: cmd.SubmittedBy,
		}
		_, err := rtRepo.Create(tx, reviewTask)
		if err != nil {
			return fmt.Errorf("could not create review tasks: %w", err)
		}
	}

	data := db.ApprovalTaskData{
		DID:       cmd.DID,
		CreatedBy: cmd.SubmittedBy,
		Approver:  *cmd.Approver,
		State:     reviewtaskstate.Open.String(),
	}
	_, err := atRepo.Create(tx, data)
	if err != nil {
		return fmt.Errorf("could not create approval task: %w", err)
	}

	return nil
}

func (h *Submitter) Handle(cmd SubmitCmd) error {

	ctx, cancel := context.WithTimeout(h.Ctx, conf.TransactionTimeout())
	defer cancel()

	tx, err := h.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	processData, err := h.CTRepo.ReadProcessData(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not process core data: %w", err)
	}

	if cmd.UpdatedAt.Unix() < processData.UpdatedAt.Unix() {
		return errors.New("contract template was updated elsewhere, please reload")
	}

	var nextTemplateState contracttemplatestate.ContractTemplateState
	if processData.State == contracttemplatestate.Draft.String() {

		if cmd.SubmittedBy != processData.CreatedBy {
			return errors.New("invalid user")
		}

		if len(cmd.Reviewer) == 0 {
			return errors.New("no reviewer provided")
		}

		if cmd.Approver == nil || len(*cmd.Approver) == 0 {
			return errors.New("no approver provided")
		}

		err := createTasks(tx, h.RTRepo, h.ATRepo, cmd)
		if err != nil {
			return err
		}

		nextTemplateState = contracttemplatestate.Submitted

	} else if processData.State == contracttemplatestate.Rejected.String() {

		if cmd.SubmittedBy != processData.CreatedBy {
			return errors.New("invalid user")
		}

		err := h.RTRepo.ReopenTasks(tx, cmd.DID)
		if err != nil {
			return errors.New("could not reopen review tasks")
		}

		err = h.ATRepo.ReopenTasks(tx, cmd.DID)
		if err != nil {
			return errors.New("could not reopen approval tasks")
		}

		nextTemplateState = contracttemplatestate.Submitted

	} else if processData.State == contracttemplatestate.Submitted.String() {

		isValid, err := h.RTRepo.IsValidReviewer(tx, processData.DID, cmd.SubmittedBy)
		if err != nil {
			return err
		}

		if !isValid {
			return errors.New("invalid user")
		}

		if cmd.ActionFlag != nil {
			if *cmd.ActionFlag == actionflag.Approval {

				exist, err := h.RTRepo.TaskExistsInState(tx, processData.DID, cmd.SubmittedBy, reviewtaskstate.Open.String())
				if err != nil {
					return err
				}

				if exist {
					return errors.New("contract template needs to be verified before")
				}

				err = h.RTRepo.UpdateState(tx, processData.DID, cmd.SubmittedBy, contracttemplatestate.Approved.String())
				if err != nil {
					return fmt.Errorf("could not update review task: %w", err)
				}

				existOpenTasks, err := h.RTRepo.AnyTasksInState(tx, processData.DID, reviewtaskstate.Open.String(), reviewtaskstate.Verified.String())
				if err != nil {
					return fmt.Errorf("could not check if review task exists: %w", err)
				}

				if !existOpenTasks {
					nextTemplateState = contracttemplatestate.Reviewed
				}

			} else if *cmd.ActionFlag == actionflag.Draft {

				err = h.RTRepo.ReopenTasks(tx, cmd.DID)
				if err != nil {
					return err
				}

				err = h.ATRepo.ReopenTasks(tx, cmd.DID)
				if err != nil {
					return err
				}

				nextTemplateState = contracttemplatestate.Rejected
			}
		} else {
			return errors.New("action flags is missing")
		}

	} else if processData.State == contracttemplatestate.Reviewed.String() {

		isValid, err := h.ATRepo.IsValidApprover(tx, processData.DID, cmd.SubmittedBy)
		if err != nil {
			return err
		}

		if !isValid {
			return errors.New("invalid user")
		}

		err = h.RTRepo.ReopenTasks(tx, cmd.DID)
		if err != nil {
			return err
		}

		err = h.ATRepo.ReopenTasks(tx, cmd.DID)
		if err != nil {
			return err
		}

		nextTemplateState = contracttemplatestate.Submitted

	} else {
		return errors.New("current contract template state is invalid")
	}

	if len(nextTemplateState) > 0 && processData.State != nextTemplateState.String() {
		err = h.CTRepo.UpdateState(tx, cmd.DID, nextTemplateState.String())
		if err != nil {
			return fmt.Errorf("could not update contract template state: %w", err)
		}

		evt := templateevents.SubmitEvent{
			DID:            cmd.DID,
			DocumentNumber: processData.DocumentNumber,
			Version:        processData.Version,
			SubmittedBy:    cmd.SubmittedBy,
			PreviousState:  processData.State,
			NewState:       nextTemplateState.String(),
			ActionFlag:     cmd.ActionFlag,
			Comments:       cmd.Comments,
			OccurredAt:     time.Now(),
		}
		err = event.Create(ctx, tx, evt, componenttype.ContractTemplateRepo)
		if err != nil {
			return fmt.Errorf("could not create event: %w", err)
		}
	}

	return tx.Commit()
}
