package jades

import (
	"encoding/json"

	"github.com/gowebpki/jcs"
)

// SettlementStatementType is the @type every settlement statement payload
// carries. Both sides re-derive the whole payload from its own decoded fields
// before believing it (service.verifySettlementStatement), so a contract
// signature JAdES — whose payload is BuildContractPayload's disjoint shape —
// can never be replayed as a settlement, nor a settlement as a signature.
const SettlementStatementType = "dcs:ContractSettlementStatement"

// BuildSettlementPayload canonicalizes a party's statement that its own
// workflow reached the settlement milestone over a named contract document,
// with the JSON Canonicalization Scheme (RFC 8785) exactly as
// BuildContractPayload does.
//
// documentHash — not contractVersion — is what binds the statement to a
// version of the document. contract_version is a per-instance receipt counter
// (contractworkflowengine/command/receivepdf.go bumps it on every ship
// received, so the two instances' counters diverge structurally) and is
// carried for audit only; documentHash is sha256 over contract_data as
// persisted, which both instances compute over the same bytes.
//
// settledAt is passed as the already-formatted RFC 3339 string that goes on
// the wire so the receiver re-derives from the exact bytes it decoded rather
// than from a reformatted time.Time.
func BuildSettlementPayload(did string, contractVersion int, settledBy, documentHash, settledAt string) ([]byte, error) {
	payload := map[string]any{
		"@type":                    SettlementStatementType,
		"dcs:contractDid":          did,
		"dcs:contractVersion":      contractVersion,
		"dcs:contractDocumentHash": documentHash,
		"dcs:settledBy":            settledBy,
		"dcs:settledAt":            settledAt,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return jcs.Transform(encoded)
}

// SettlementClaim is the decoded settlement statement payload. Fields are read
// out of the VERIFIED JAdES bytes and then re-canonicalized back into the
// payload they claim to be, so nothing outside these five may travel.
type SettlementClaim struct {
	Type            string `json:"@type"`
	ContractDid     string `json:"dcs:contractDid"`
	ContractVersion int    `json:"dcs:contractVersion"`
	DocumentHash    string `json:"dcs:contractDocumentHash"`
	SettledBy       string `json:"dcs:settledBy"`
	SettledAt       string `json:"dcs:settledAt"`
}
