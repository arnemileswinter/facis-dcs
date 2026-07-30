package dcstodcs

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

func contractWithData(t *testing.T, jsonld string) *db.Contract {
	t.Helper()
	payload := datatype.JSON(jsonld)
	return &db.Contract{ContractData: &payload}
}

// regeneratorHash is the hash pdfgeneration/event.Subscriber.appendC2PA records
// against the render it just stored. The ship gate compares against it, so the
// two must be computed over the same bytes the same way.
func regeneratorHash(jsonld string) string {
	sum := sha256.Sum256([]byte(jsonld))
	return hex.EncodeToString(sum[:])
}

func TestShipDefersAPDFRenderedFromASupersededDocument(t *testing.T) {
	contract := contractWithData(t, `{"dcs:documentStructure":{},"dcs:contractFields":[{"dcs:value":"8"}]}`)
	pdfState := &db.ContractPDFState{
		IPFSCID:     "QmStale",
		C2PAState:   "draft",
		PayloadHash: regeneratorHash(`{"dcs:documentStructure":{},"dcs:contractFields":[{}]}`),
	}

	require.True(t, holdsSupersededPDF(pdfState, contract),
		"a PDF rendered before the fields were filled must not be shipped: the receiver adopts the "+
			"document carried in it as its own copy")
}

func TestShipProceedsWhenThePDFMatchesTheStoredDocument(t *testing.T) {
	jsonld := `{"dcs:documentStructure":{},"dcs:contractFields":[{"dcs:value":"8"}]}`
	pdfState := &db.ContractPDFState{
		IPFSCID:     "QmCurrent",
		C2PAState:   "draft",
		PayloadHash: regeneratorHash(jsonld),
	}

	require.False(t, holdsSupersededPDF(pdfState, contractWithData(t, jsonld)))
}

func TestShipProceedsForAFrozenArtifactWhoseHashCameFromSigning(t *testing.T) {
	// Signing records the hash of the document it signed and the artifact is
	// never re-rendered afterwards, so a mismatch here can never resolve —
	// deferring on it would strand every signed contract's ship.
	pdfState := &db.ContractPDFState{
		IPFSCID:     "QmSigned",
		C2PAState:   "active",
		PayloadHash: regeneratorHash(`{"signed":"under a different byte order"}`),
	}

	require.False(t, holdsSupersededPDF(pdfState, contractWithData(t, `{"dcs:documentStructure":{}}`)))
}

func TestShipProceedsWhenNoPayloadHashWasEverRecorded(t *testing.T) {
	pdfState := &db.ContractPDFState{IPFSCID: "QmLegacy", C2PAState: "draft"}

	require.False(t, holdsSupersededPDF(pdfState, contractWithData(t, `{"dcs:documentStructure":{}}`)))
}

func TestSupersededCheckHashesAContractWithoutDataLikeTheRegenerator(t *testing.T) {
	require.Equal(t, regeneratorHash(""), contractDataHash(&db.Contract{}))
}
