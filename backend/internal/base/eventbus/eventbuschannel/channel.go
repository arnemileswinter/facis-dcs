package eventbuschannel

import (
	"fmt"
	"strings"
)

type EventBusChannel string

const (
	ContractWorkflowEngine EventBusChannel = "CONTRACT_WORKFLOW_ENGINE"
)

var validFlag = map[EventBusChannel]bool{
	ContractWorkflowEngine: true,
}

func NewEventBusChannel(s string) (EventBusChannel, error) {
	flag := EventBusChannel(strings.ToUpper(s))
	if !flag.IsValid() {
		return "", fmt.Errorf("invalid action flag: %s", s)
	}
	return flag, nil
}

// IsValid checks if the EventBusChannel is a valid role
func (f EventBusChannel) IsValid() bool {
	upper := EventBusChannel(strings.ToUpper(string(f)))
	return validFlag[upper]
}

// String returns the string representation of the EventBusChannel
func (f EventBusChannel) String() string {
	return string(f)
}
