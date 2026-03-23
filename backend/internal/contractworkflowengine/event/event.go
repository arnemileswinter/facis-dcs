package event

import (
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/eventtype"
	"time"
)

// CreateEvent is emitted when a new contract is created.
type CreateEvent struct {
	DID          string         `json:"did"`
	TemplateDID  string         `json:"template_did"`
	CreatedBy    string         `json:"created_by"`
	Name         *string        `json:"name"`
	Description  *string        `json:"description"`
	ContractData *datatype.JSON `json:"contract_data"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

// EventType implements the Event interface.
func (e CreateEvent) EventType() string {
	return eventtype.Create.String()
}

// GetDID implements the Event interface.
func (e CreateEvent) GetDID() string {
	return e.DID
}

// UpdateEvent is emitted when contract data is updated.
type UpdateEvent struct {
	DID                string         `json:"did"`
	UpdatedBy          string         `json:"updated_by"`
	OldContractVersion *int           `json:"old_contract_version,omitempty"`
	NewContractVersion *int           `json:"new_contract_version,omitempty"`
	OldName            *string        `json:"old_name,omitempty"`
	NewName            *string        `json:"new_name,omitempty"`
	OldDescription     *string        `json:"old_description,omitempty"`
	NewDescription     *string        `json:"new_description,omitempty"`
	OldContractData    *datatype.JSON `json:"old_contract_data,omitempty"`
	NewContractData    *datatype.JSON `json:"new_contract_data,omitempty"`
	OccurredAt         time.Time      `json:"occurred_at"`
}

// EventType implements the Event interface.
func (e UpdateEvent) EventType() string {
	return eventtype.Update.String()
}

// GetDID implements the Event interface.
func (e UpdateEvent) GetDID() string {
	return e.DID
}

// SubmitEvent is emitted when a contract is submitted
type SubmitEvent struct {
	DID           string    `json:"did"`
	PreviousState string    `json:"previous_state"`
	NewState      string    `json:"new_state"`
	SubmittedBy   string    `json:"submitted_by"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// EventType implements the Event interface.
func (e SubmitEvent) EventType() string {
	return eventtype.Submit.String()
}

// GetDID implements the Event interface.
func (e SubmitEvent) GetDID() string {
	return e.DID
}

// RetrieveByIDEvent is emitted when contract data is retrieved.
type RetrieveByIDEvent struct {
	DID         string    `json:"did"`
	RetrievedBy string    `json:"retrieved_by"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// EventType implements the Event interface.
func (e RetrieveByIDEvent) EventType() string {
	return eventtype.RetrieveByID.String()
}

// GetDID implements the Event interface.
func (e RetrieveByIDEvent) GetDID() string {
	return e.DID
}

// RetrieveAllEvent is emitted when contract data is retrieved.
type RetrieveAllEvent struct {
	RetrievedBy string    `json:"retrieved_by"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// EventType implements the Event interface.
func (e RetrieveAllEvent) EventType() string {
	return eventtype.RetrieveAll.String()
}

// GetDID implements the Event interface.
func (e RetrieveAllEvent) GetDID() string {
	return "*"
}
