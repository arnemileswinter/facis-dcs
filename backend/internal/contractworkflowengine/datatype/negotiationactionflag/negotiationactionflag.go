package negotiationactionflag

import (
	"fmt"
	"strings"
)

type NegotiationActionFlag string

const (
	Accepting NegotiationActionFlag = "ACCEPTING"
	Rejecting NegotiationActionFlag = "REJECTING"
)

var validFlag = map[NegotiationActionFlag]bool{
	Accepting: true,
	Rejecting: true,
}

func NewNegotiationActionFlag(s string) (NegotiationActionFlag, error) {
	flag := NegotiationActionFlag(strings.ToUpper(s))
	if !flag.IsValid() {
		return "", fmt.Errorf("invalid negotiation action flag: %s", s)
	}
	return flag, nil
}

// IsValid checks if the NegotiationActionFlag is a valid role
func (f NegotiationActionFlag) IsValid() bool {
	upper := NegotiationActionFlag(strings.ToUpper(string(f)))
	return validFlag[upper]
}

// String returns the string representation of the NegotiationActionFlag
func (f NegotiationActionFlag) String() string {
	return string(f)
}
