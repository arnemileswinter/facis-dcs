package serviceoffering

import (
	"fmt"
	"strings"
)

const (
	participantIDSegment     = "participant"
	serviceOfferingIDSegment = "service-offering"
)

// BuildID converts a participant DID to its corresponding service offering DID.
func BuildID(participantID string) (string, error) {
	if participantID == "" {
		return "", fmt.Errorf("participant id is empty")
	}
	if !strings.Contains(participantID, participantIDSegment) {
		return "", fmt.Errorf("participant id does not contain %q", participantIDSegment)
	}

	serviceOfferingID := strings.ReplaceAll(participantID, participantIDSegment, serviceOfferingIDSegment)
	return serviceOfferingID, nil
}
