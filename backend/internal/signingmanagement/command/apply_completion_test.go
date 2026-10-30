package command

import (
	"testing"

	"digital-contracting-service/internal/signingmanagement/db"
)

func signedRecord(field string) db.SignatureRecord {
	return db.SignatureRecord{Status: "SIGNED", FieldName: &field}
}

// DCS-FR-CSA-08, DCS-FR-CWE-20: the archive entry is written once, by the
// signature that completes the signatures this instance holds, so it carries
// all of them.
func TestSigningCompletesOnlyWithTheLastLocalSignature(t *testing.T) {
	fields := []string{"signer-1", "signer-2"}

	if signingCompletes(fields, []db.SignatureRecord{signedRecord("signer-1")}, "signer-1") {
		t.Error("the first of two signatures must not complete signing")
	}
	if !signingCompletes(fields, []db.SignatureRecord{signedRecord("signer-1"), signedRecord("signer-2")}, "signer-2") {
		t.Error("the second of two signatures must complete signing")
	}
	revoked := signedRecord("signer-2")
	revoked.Status = "REVOKED"
	if signingCompletes(fields, []db.SignatureRecord{signedRecord("signer-1"), revoked}, "signer-1") {
		t.Error("a revoked signature does not count")
	}

	// A contract that declares no field completes with its first signature.
	if !signingCompletes(nil, []db.SignatureRecord{{Status: "SIGNED"}}, "") {
		t.Error("the first signature of a contract without fields must complete signing")
	}
	if signingCompletes(nil, []db.SignatureRecord{{Status: "SIGNED"}, {Status: "SIGNED"}}, "") {
		t.Error("a later signature of a contract without fields must not archive again")
	}
}

// The counterparty's slot is signed in the peer's deployment, so only the
// local slots decide when this instance's signing completes.
func TestLocalSignatureFieldsLeaveOutTheCounterpartySlot(t *testing.T) {
	responsible := &db.Responsible{Creator: instanceA, Counterparty: instanceB}
	local := localSignatureFields([]string{instanceA, instanceB}, responsible, instanceA)
	if len(local) != 1 || local[0] != instanceA {
		t.Fatalf("want only the own slot, got %v", local)
	}
	if !signingCompletes(local, []db.SignatureRecord{signedRecord(instanceA)}, instanceA) {
		t.Error("the own signature completes this instance's signing of a federated contract")
	}
}
