package command

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvidenceIntegrityAndLimits(t *testing.T) {
	hash := sha256.Sum256([]byte("original"))
	e := Evidence{EvidenceID: "evidence", DID: "contract", Content: []byte("original"), SHA256: hex.EncodeToString(hash[:]), Filename: "name", Description: "description"}
	require.NoError(t, e.Verify("contract", "evidence"))
	require.Error(t, e.Verify("another-contract", "evidence"))
	require.Error(t, e.Verify("contract", "another-evidence"))
	e.Content = []byte("changed")
	require.Error(t, e.Verify("contract", "evidence"))
	t.Setenv("DCS_EVIDENCE_MAX_TEXT_BYTES", "4")
	require.ErrorIs(t, e.Validate(), ErrInvalidEvidence)
}
