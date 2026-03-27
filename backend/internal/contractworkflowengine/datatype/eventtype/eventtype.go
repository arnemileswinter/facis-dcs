package eventtype

import (
	"fmt"
	"strings"
)

type EventType string

const (
	Create       EventType = "CREATE_CONTRACT"
	Submit       EventType = "SUBMIT_CONTRACT"
	Negotiation  EventType = "NEGOTIATE_CONTRACT"
	Response     EventType = "RESPONSE_CONTRACT"
	Approve      EventType = "APPROVE_CONTRACT"
	Reject       EventType = "REJECT_CONTRACT"
	Verify       EventType = "VERIFY_CONTRACT"
	Update       EventType = "UPDATE_CONTRACT"
	RetrieveAll  EventType = "RETRIEVE_ALL_CONTRACTS"
	RetrieveByID EventType = "RETRIEVE_CONTRACT_BY_ID"
	Search       EventType = "SEARCH_CONTRACT"
	Archive      EventType = "ARCHIVE_CONTRACT"
)

var validStates = map[EventType]bool{
	Create:       true,
	Submit:       true,
	Negotiation:  true,
	Response:     true,
	Approve:      true,
	Reject:       true,
	Verify:       true,
	Update:       true,
	RetrieveAll:  true,
	RetrieveByID: true,
	Search:       true,
	Archive:      true,
}

func NewEventType(s string) (EventType, error) {
	ts := EventType(strings.ToUpper(s))
	if !ts.IsValid() {
		return "", fmt.Errorf("invalid event type: %s", s)
	}
	return ts, nil
}

// IsValid checks if the EventType is a valid role
func (s EventType) IsValid() bool {
	upper := EventType(strings.ToUpper(string(s)))
	return validStates[upper]
}

// String returns the string representation of the EventType
func (s EventType) String() string {
	return string(s)
}
