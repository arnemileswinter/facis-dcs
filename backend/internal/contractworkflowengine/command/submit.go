package command

import (
	"context"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/contractworkflowengine/datatype/actionflag"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/reviewtaskstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
	contractevents "digital-contracting-service/internal/contractworkflowengine/event"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type SubmitCmd struct {
	DID         string
	UpdatedAt   time.Time
	SubmittedBy string
	Reviewer    []string
	Approver    *string
	ActionFlag  *actionflag.ActionFlag
	Comments    []string
}

type Submitter struct {
	Ctx    context.Context
	DB     *sqlx.DB
	CRepo  db.ContractRepo
	RTRepo db.ReviewTaskRepo
	ATRepo db.ApprovalTaskRepo
	NRepo  db.NegotiationRepo
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

	processData, err := h.CRepo.ReadProcessData(tx, cmd.DID)
	if err != nil {
		return fmt.Errorf("could not read process data: %w", err)
	}

	if cmd.UpdatedAt.Before(processData.UpdatedAt) {
		return errors.New("contract was updated elsewhere, please reload")
	}

	var nextState contractstate.ContractState
	if processData.State == contractstate.Draft.String() {

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

		nextState = contractstate.Negotiation

	} else if processData.State == contractstate.Negotiation.String() {

		isValidCounterpart, err := h.NRepo.IsValidCounterpart(tx, processData.DID, processData.ContractVersion, cmd.SubmittedBy)
		if err != nil {
			return fmt.Errorf("could not validate counterpart: %w", err)
		}

		if cmd.SubmittedBy != processData.CreatedBy && isValidCounterpart == false {
			return errors.New("invalid user")
		}

		hasOpenNegotiations, err := h.NRepo.HasOpenNegotiationDecisions(tx, cmd.DID, processData.ContractVersion)
		if err != nil {
			return fmt.Errorf("could not check open negotiations: %w", err)
		}

		if hasOpenNegotiations {
			return errors.New("not all negations are processed")
		}

		nextState = contractstate.Submitted

	} else if processData.State == contractstate.Submitted.String() {

		if cmd.ActionFlag != nil {
			if *cmd.ActionFlag == actionflag.Approval {

				valid, err := h.RTRepo.IsValidReviewer(tx, processData.DID, cmd.SubmittedBy)
				if err != nil {
					return err
				}

				if !valid {
					return errors.New("invalid user")
				}

				err = h.RTRepo.UpdateState(tx, processData.DID, cmd.SubmittedBy, contractstate.Approved.String())
				if err != nil {
					return fmt.Errorf("could not update approval task: %w", err)
				}

				existOpenTasks, err := h.RTRepo.AnyTasksInState(tx, processData.DID, reviewtaskstate.Open.String())
				if err != nil {
					return fmt.Errorf("could not check if review task exists: %w", err)
				}

				if !existOpenTasks {
					nextState = contractstate.Reviewed
				}

			} else if *cmd.ActionFlag == actionflag.Reject {

				isValid, err := h.RTRepo.IsValidReviewer(tx, processData.DID, cmd.SubmittedBy)
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

				nextState = contractstate.Negotiation
			}

		} else {
			return errors.New("action flags is missing")
		}

	} else if processData.State == contractstate.Reviewed.String() {

		isValid, err := h.ATRepo.IsValidApprover(tx, processData.DID, cmd.SubmittedBy)
		if err != nil {
			return err
		}

		if !isValid {
			return errors.New("invalid user")
		}

		nextState = contractstate.Approved

	} else {
		return errors.New("current contract state is invalid")
	}

	if len(nextState) > 0 && processData.State != nextState.String() {
		err = h.CRepo.UpdateState(tx, cmd.DID, nextState.String())
		if err != nil {
			return fmt.Errorf("could not update contract state: %w", err)
		}

		evt := contractevents.SubmitEvent{
			DID:             cmd.DID,
			ContractVersion: processData.ContractVersion,
			SubmittedBy:     cmd.SubmittedBy,
			PreviousState:   processData.State,
			NewState:        nextState.String(),
			ActionFlag:      cmd.ActionFlag,
			Comments:        cmd.Comments,
			OccurredAt:      time.Now(),
		}
		err = event.Create(ctx, tx, evt, componenttype.ContractWorkflowEngine)
		if err != nil {
			return fmt.Errorf("could not create event: %w", err)
		}
	}

	return tx.Commit()
}
