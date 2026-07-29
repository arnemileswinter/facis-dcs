package oid4vp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PIDVCT is the German EUDI PID credential type — the real one, asserting an
// identity that was actually proofed.
//
// A deployment served by a demo issuer must NOT request this: no identity
// proofing happens there, so accepting it under this type would claim an
// assurance level nobody established. Point OID4VP_PID_DCQL_QUERY at the demo
// type instead (urn:dcs:pid:demo:v1), which is what the demo issuer mints.
const PIDVCT = "urn:eudi:pid:de:1"

// DemoPIDVCT is the type the bundled demo PID issuer mints. It is deliberately
// not an EUDI type: it describes a person nobody verified.
const DemoPIDVCT = "urn:dcs:pid:demo:v1"

const PIDCredentialQueryID = "eudi_pid_credential"

// DefaultPIDDCQLQuery requests a dc+sd-jwt PID credential for identity presentation.
// Override the full query via OID4VP_PID_DCQL_QUERY when needed.
func DefaultPIDDCQLQuery() map[string]any {
	return map[string]any{
		"credentials": []any{
			map[string]any{
				"id":     PIDCredentialQueryID,
				"format": "dc+sd-jwt",
				"meta": map[string]any{
					"vct_values": []string{PIDVCT},
				},
				"require_cryptographic_holder_binding": true,
			},
		},
	}
}

func LoadPIDDCQLQuery(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultPIDDCQLQuery(), nil
	}

	var q any
	err := json.Unmarshal([]byte(raw), &q)
	if err != nil {
		return nil, fmt.Errorf("invalid OID4VP_PID_DCQL_QUERY JSON: %w", err)
	}

	return q, nil
}
