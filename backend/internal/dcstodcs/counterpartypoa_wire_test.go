package dcstodcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	smdb "digital-contracting-service/internal/signingmanagement/db"
)

// A Power of Attorney must never leave here without the summary attesting the
// signature it stands behind: the receiver refuses one that arrives alone, so a
// signing branch that retains no summary silently makes every ship from this
// instance unacceptable to its peer. Nothing covered the producer side, which is
// how exactly that shipped.
func TestEveryShippedPoACarriesItsSummary(t *testing.T) {
	applied := []smdb.AppliedPoA{
		{FieldName: "did:web:a.example", SignerDID: "did:jwk:one", Presentation: "p1", SummaryVC: `{"type":["ContractSigningSummaryCredential"]}`},
		{FieldName: "did:web:b.example", SignerDID: "did:jwk:two", Presentation: "p2", SummaryVC: ""},
	}

	evidence := make([]SignatoryPoA, 0, len(applied))
	for _, poa := range applied {
		evidence = append(evidence, SignatoryPoA{Party: poa.FieldName, Presentation: poa.Presentation, Summary: poa.SummaryVC})
	}
	wire := WireSignatoryPoAs(evidence)
	require.Len(t, wire, 2)

	assert.NotNil(t, wire[0].Summary, "a retained summary must reach the wire")
	assert.Nil(t, wire[1].Summary, "a ceremony with no retained summary ships none, and the peer will refuse it")

	// What the receiver reconstructs must be what the shipper sent.
	back := ReceivedSignatoryPoAs(wire)
	require.Len(t, back, 2)
	assert.Equal(t, applied[0].SummaryVC, back[0].Summary)
	assert.Empty(t, back[1].Summary)
}
